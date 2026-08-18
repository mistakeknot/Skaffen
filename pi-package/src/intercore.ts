import { isAbsolute, relative, resolve } from "node:path";
import type {
	AgencySummary,
	ExecResult,
	HarnessState,
	IntercoreProvenance,
	IntercoreVersion,
	ProducerSummary,
	RunSummary,
	SituationSnapshot,
} from "./types.ts";

export type IcRunner = (
	args: readonly string[],
	options: { cwd: string; timeout: number },
) => Promise<ExecResult>;

interface CommandOutcome {
	result?: ExecResult;
	error?: Error;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNonnegativeSafeInteger(value: unknown): value is number {
	return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}

function isProducerSummary(value: unknown): value is ProducerSummary {
	if (!isRecord(value)) return false;
	return (
		typeof value.kind === "string" &&
		typeof value.name === "string" &&
		(value.class === undefined || typeof value.class === "string") &&
		(value.version === undefined || typeof value.version === "string")
	);
}

function isAgencySummary(value: unknown): value is AgencySummary {
	if (!isRecord(value)) return false;
	return (
		typeof value.name === "string" &&
		typeof value.cycle_id === "string" &&
		typeof value.stage === "string" &&
		typeof value.event_type === "string" &&
		(value.run_id === undefined || typeof value.run_id === "string") &&
		(value.project_dir === undefined || typeof value.project_dir === "string") &&
		isNonnegativeSafeInteger(value.last_event_id) &&
		typeof value.updated_at === "string"
	);
}

function isRunSummary(value: unknown): value is RunSummary {
	if (!isRecord(value)) return false;
	return (
		typeof value.id === "string" &&
		typeof value.phase === "string" &&
		(value.oodarc_role === undefined || typeof value.oodarc_role === "string") &&
		typeof value.status === "string" &&
		typeof value.project_dir === "string" &&
		typeof value.goal === "string" &&
		isNonnegativeSafeInteger(value.created_at) &&
		(value.producer === undefined || isProducerSummary(value.producer))
	);
}

function isSituationSnapshot(value: unknown): value is SituationSnapshot {
	if (!isRecord(value)) return false;
	if (typeof value.timestamp !== "string" || !Array.isArray(value.runs) || !value.runs.every(isRunSummary)) {
		return false;
	}
	if (!isRecord(value.dispatches)) return false;
	if (
		!isNonnegativeSafeInteger(value.dispatches.active) ||
		!isNonnegativeSafeInteger(value.dispatches.total) ||
		!Array.isArray(value.dispatches.agents)
	) {
		return false;
	}
	if (!Array.isArray(value.recent_events) || !isRecord(value.queue)) return false;
	if (value.agencies !== undefined) {
		if (!Array.isArray(value.agencies) || !value.agencies.every(isAgencySummary)) return false;
	}
	if (
		!isNonnegativeSafeInteger(value.queue.pending) ||
		!isNonnegativeSafeInteger(value.queue.running) ||
		!isNonnegativeSafeInteger(value.queue.retrying)
	) {
		return false;
	}
	if (value.budget !== undefined) {
		if (!isRecord(value.budget)) return false;
		if (
			typeof value.budget.run_id !== "string" ||
			!isNonnegativeSafeInteger(value.budget.budget) ||
			!isNonnegativeSafeInteger(value.budget.used) ||
			!isNonnegativeSafeInteger(value.budget.remaining)
		) {
			return false;
		}
	}
	return true;
}

function isIntercoreVersion(value: unknown): value is IntercoreVersion {
	if (!isRecord(value)) return false;
	return (
		typeof value.version === "string" &&
		(value.commit === undefined || typeof value.commit === "string") &&
		(value.commit_time === undefined || typeof value.commit_time === "string") &&
		typeof value.dirty === "boolean" &&
		typeof value.go === "string" &&
		typeof value.os === "string" &&
		typeof value.arch === "string" &&
		typeof value.source === "string" &&
		(value.schema === undefined || isNonnegativeSafeInteger(value.schema))
	);
}

export function parseIntercoreVersion(stdout: string): IntercoreVersion | undefined {
	if (!stdout.trim()) return undefined;
	try {
		const value: unknown = JSON.parse(stdout);
		return isIntercoreVersion(value) ? value : undefined;
	} catch {
		return undefined;
	}
}

export function parseSituation(stdout: string): SituationSnapshot | undefined {
	if (!stdout.trim()) return undefined;
	try {
		const value: unknown = JSON.parse(stdout);
		return isSituationSnapshot(value) ? value : undefined;
	} catch {
		return undefined;
	}
}

function isWithin(projectDir: string, cwd: string): boolean {
	if (!projectDir) return false;
	const project = resolve(projectDir);
	const candidate = resolve(cwd);
	const pathFromProject = relative(project, candidate);
	return pathFromProject === "" || (!pathFromProject.startsWith("..") && !isAbsolute(pathFromProject));
}

export function findCurrentRun(snapshot: SituationSnapshot, cwd: string): RunSummary | undefined {
	return snapshot.runs
		.filter((run) => isWithin(run.project_dir, cwd))
		.sort((left, right) => resolve(right.project_dir).length - resolve(left.project_dir).length)[0];
}

async function runCommand(
	runner: IcRunner,
	args: readonly string[],
	options: { cwd: string; timeout: number },
): Promise<CommandOutcome> {
	try {
		return { result: await runner(args, options) };
	} catch (error) {
		const normalized = error instanceof Error ? error : new Error(String(error));
		if (normalized.name === "AbortError") throw normalized;
		return { error: normalized };
	}
}

function diagnosticFromStderr(stderr: string): string | undefined {
	const lines = stderr
		.split("\n")
		.map((line) => line.trim())
		.filter(Boolean)
		.reverse();
	for (const line of lines) {
		try {
			const value: unknown = JSON.parse(line);
			if (isRecord(value)) {
				if (typeof value.error === "string" && value.error.trim()) return value.error.trim();
				if (typeof value.msg === "string" && value.msg.trim()) return value.msg.trim();
			}
		} catch {
			// Fall back to the raw diagnostic below.
		}
	}
	return lines[0];
}

function commandFailure(label: string, outcome: CommandOutcome): string | undefined {
	if (outcome.error) return outcome.error.message;
	const command = outcome.result;
	if (!command) return `${label} unavailable`;
	if (command.killed) return `${label} timed out`;
	if (command.code !== 0) return diagnosticFromStderr(command.stderr) ?? `${label} exited ${command.code}`;
	return undefined;
}

function inspectProvenance(outcome: CommandOutcome): {
	provenance: IntercoreProvenance;
	version?: IntercoreVersion;
	provenanceReason?: string;
} {
	const failure = commandFailure("ic version", outcome);
	if (failure) return { provenance: "unavailable", provenanceReason: failure };

	const version = outcome.result ? parseIntercoreVersion(outcome.result.stdout) : undefined;
	if (!version) return { provenance: "unavailable", provenanceReason: "invalid ic version response" };
	if (version.dirty) return { provenance: "unverified", version, provenanceReason: "Intercore build is dirty" };
	if (!version.commit?.trim()) {
		return { provenance: "unverified", version, provenanceReason: "Intercore build commit is unavailable" };
	}
	if (!version.source.trim() || version.source === "unknown") {
		return { provenance: "unverified", version, provenanceReason: "Intercore build source is unknown" };
	}
	return { provenance: "verified", version };
}

function bothCommandsUnavailable(health: CommandOutcome, situation: CommandOutcome): boolean {
	const noUsableSignal = (outcome: CommandOutcome): boolean => {
		if (outcome.error) return true;
		if (!outcome.result) return true;
		if (outcome.result.code === 127) return true;
		return (
			outcome.result.code !== 0 &&
			!outcome.result.killed &&
			!outcome.result.stdout.trim() &&
			!outcome.result.stderr.trim()
		);
	};
	return noUsableSignal(health) && noUsableSignal(situation);
}

export async function inspectIntercore(
	runner: IcRunner,
	cwd: string,
	timeout = 250,
): Promise<HarnessState> {
	const started = performance.now();
	const options = { cwd, timeout };
	const [healthOutcome, versionOutcome, situationOutcome] = await Promise.all([
		runCommand(runner, ["health", "--json"], options),
		runCommand(runner, ["version", "--json"], options),
		runCommand(runner, ["situation", "snapshot", "--json"], options),
	]);

	const healthResult = healthOutcome.result;
	const situationResult = situationOutcome.result;
	const healthOkay =
		healthResult !== undefined &&
		!healthResult.killed &&
		healthResult.code === 0 &&
		healthResult.stdout.trim() === "ok";
	const snapshot =
		situationResult !== undefined && !situationResult.killed && situationResult.code === 0
			? parseSituation(situationResult.stdout)
			: undefined;

	let health: HarnessState["health"];
	let reason: string | undefined;
	if (healthOkay && snapshot) {
		health = "healthy";
	} else if (bothCommandsUnavailable(healthOutcome, situationOutcome)) {
		health = "unavailable";
		const diagnostic = commandFailure("ic", healthOutcome) ?? commandFailure("ic", situationOutcome);
		reason = diagnostic ? `ic unavailable: ${diagnostic}` : "ic unavailable";
	} else {
		health = "degraded";
		reason =
			commandFailure("ic health", healthOutcome) ??
			commandFailure("ic situation snapshot", situationOutcome) ??
			(!healthOkay ? "invalid ic health response" : "invalid situation snapshot");
		if (!snapshot && situationResult?.code === 0 && !situationResult.killed) {
			reason = "invalid situation snapshot";
		}
	}

	const provenance = inspectProvenance(versionOutcome);
	return {
		health,
		...(reason ? { reason } : {}),
		...provenance,
		checkedAt: new Date().toISOString(),
		latencyMs: Math.max(0, Math.round(performance.now() - started)),
		...(snapshot ? { snapshot, currentRun: findCurrentRun(snapshot, cwd) } : {}),
	};
}
