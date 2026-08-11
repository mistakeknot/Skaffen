import { isAbsolute, relative, resolve } from "node:path";
import type { ExecResult, HarnessState, RunSummary, SituationSnapshot } from "./types.ts";

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

function isFiniteNumber(value: unknown): value is number {
	return typeof value === "number" && Number.isFinite(value);
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
		isFiniteNumber(value.created_at)
	);
}

function isSituationSnapshot(value: unknown): value is SituationSnapshot {
	if (!isRecord(value)) return false;
	if (typeof value.timestamp !== "string" || !Array.isArray(value.runs) || !value.runs.every(isRunSummary)) {
		return false;
	}
	if (!isRecord(value.dispatches)) return false;
	if (
		!isFiniteNumber(value.dispatches.active) ||
		!isFiniteNumber(value.dispatches.total) ||
		!Array.isArray(value.dispatches.agents)
	) {
		return false;
	}
	if (!Array.isArray(value.recent_events) || !isRecord(value.queue)) return false;
	if (
		!isFiniteNumber(value.queue.pending) ||
		!isFiniteNumber(value.queue.running) ||
		!isFiniteNumber(value.queue.retrying)
	) {
		return false;
	}
	if (value.budget !== undefined) {
		if (!isRecord(value.budget)) return false;
		if (
			typeof value.budget.run_id !== "string" ||
			!isFiniteNumber(value.budget.budget) ||
			!isFiniteNumber(value.budget.used) ||
			!isFiniteNumber(value.budget.remaining)
		) {
			return false;
		}
	}
	return true;
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
		return { error: error instanceof Error ? error : new Error(String(error)) };
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

function bothCommandsUnavailable(health: CommandOutcome, situation: CommandOutcome): boolean {
	const noUsableSignal = (outcome: CommandOutcome): boolean => {
		if (outcome.error) return true;
		if (!outcome.result) return true;
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
	const [healthOutcome, situationOutcome] = await Promise.all([
		runCommand(runner, ["health", "--json"], options),
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

	return {
		health,
		...(reason ? { reason } : {}),
		checkedAt: new Date().toISOString(),
		latencyMs: Math.max(0, Math.round(performance.now() - started)),
		...(snapshot ? { snapshot, currentRun: findCurrentRun(snapshot, cwd) } : {}),
	};
}
