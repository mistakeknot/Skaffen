import type { AgencySummary, HarnessState, RunSummary } from "./types.ts";

const ANSI_ESCAPE = /\u001b\[[0-?]*[ -/]*[@-~]/gu;
const CONTROL_CHARACTER = /[\u0000-\u001f\u007f-\u009f]/gu;
const MAX_SUMMARY_ITEMS = 20;

function clean(value: string, maxLength = 240): string {
	const normalized = value
		.replace(ANSI_ESCAPE, "")
		.replace(CONTROL_CHARACTER, " ")
		.replace(/\s+/gu, " ")
		.trim();
	if (normalized.length <= maxLength) return normalized;
	return `${normalized.slice(0, Math.max(0, maxLength - 1))}…`;
}

function boundedStatus(value: string): string {
	return clean(value, 80);
}

function formatVersionLines(state: HarnessState): string[] {
	if (!state.version) {
		const reason = state.provenanceReason ? ` — ${clean(state.provenanceReason)}` : "";
		return [`Provenance: ${state.provenance}${reason}`];
	}
	const schema = state.version.schema === undefined ? "schema unknown" : `schema ${state.version.schema}`;
	const commit = state.version.commit ? clean(state.version.commit, 80).slice(0, 12) : "commit unknown";
	const source = clean(state.version.source, 40) || "source unknown";
	const reason = state.provenanceReason ? ` — ${clean(state.provenanceReason)}` : "";
	return [
		`Intercore: ${clean(state.version.version, 40)} · ${schema}`,
		`Build: ${commit} · ${state.provenance} · ${source}${reason}`,
	];
}

export function formatCompactStatus(state: HarnessState): string {
	if (state.health === "unavailable") return "◇ ic offline";

	const marker = state.health === "healthy" ? "◆" : "◇!";
	if (state.currentRun) {
		const phase = clean(state.currentRun.phase, 32) || "run";
		const role = state.currentRun.oodarc_role ? clean(state.currentRun.oodarc_role, 32) : "";
		return boundedStatus(`${marker} ${phase}${role ? ` · ${role}` : ""}`);
	}

	const active = state.snapshot?.runs.length ?? 0;
	return boundedStatus(`${marker} idle${active > 0 ? ` · ${active} active` : ""}`);
}

function producerLabel(run: RunSummary): string | undefined {
	if (!run.producer) return undefined;
	const kind = clean(run.producer.kind, 48) || "unknown kind";
	const name = clean(run.producer.name, 80) || "unknown producer";
	const version = run.producer.version ? `@${clean(run.producer.version, 48)}` : "";
	return `${kind}/${name}${version}`;
}

function formatRun(run: RunSummary): string {
	const role = run.oodarc_role ? ` · ${clean(run.oodarc_role, 48)}` : "";
	const goal = clean(run.goal, 160) || "no goal";
	const project = clean(run.project_dir, 200) || "unknown project";
	const producer = producerLabel(run);
	return `- ${clean(run.id, 80)}: ${clean(run.phase, 48)}${role} — ${goal} (${project})${producer ? `; producer: ${producer}` : ""}`;
}

function formatAgency(agency: AgencySummary): string {
	const identity = `${clean(agency.name, 80)}/${clean(agency.cycle_id, 80)}`;
	const run = agency.run_id ? `; run ${clean(agency.run_id, 80)}` : "";
	const project = agency.project_dir ? ` (${clean(agency.project_dir, 160)})` : "";
	return `- ${identity}: ${clean(agency.stage, 64)} · ${clean(agency.event_type, 96)}${run}${project}`;
}

export function formatHarnessDetails(state: HarnessState): string {
	const lines = [
		"# Skaffen Harness",
		`Health: ${state.health}`,
		...(state.reason ? [`Reason: ${clean(state.reason)}`] : []),
		`Checked: ${clean(state.checkedAt, 64)}`,
		`Latency: ${Math.max(0, Math.round(state.latencyMs))} ms`,
		...formatVersionLines(state),
	];

	if (state.currentRun) {
		const role = state.currentRun.oodarc_role ? ` (${clean(state.currentRun.oodarc_role, 48)})` : "";
		const producer = producerLabel(state.currentRun);
		const producerClass = state.currentRun.producer?.class
			? ` (${clean(state.currentRun.producer.class, 64)})`
			: "";
		lines.push(
			`Current run: ${clean(state.currentRun.id, 80)}`,
			`Phase: ${clean(state.currentRun.phase, 48)}${role}`,
			`Goal: ${clean(state.currentRun.goal, 200) || "not set"}`,
			...(producer ? [`Producer: ${producer}${producerClass}`] : []),
		);
	} else {
		lines.push("Current run: none for this working directory");
	}

	return lines.join("\n");
}

export function formatSituation(state: HarnessState): string {
	if (!state.snapshot) {
		return [
			"# Intercore Situation",
			"Situation snapshot unavailable.",
			...(state.reason ? [`Reason: ${clean(state.reason)}`] : []),
		].join("\n");
	}

	const snapshot = state.snapshot;
	const orderedRuns = state.currentRun
		? [state.currentRun, ...snapshot.runs.filter((run) => run.id !== state.currentRun?.id)]
		: snapshot.runs;
	const lines = [
		"# Intercore Situation",
		`Snapshot: ${clean(snapshot.timestamp, 64)}`,
		`Health: ${state.health}`,
		...(state.reason ? [`Reason: ${clean(state.reason)}`] : []),
		...formatVersionLines(state),
		`Dispatches: ${snapshot.dispatches.active} active / ${snapshot.dispatches.total} total`,
		`Queue: ${snapshot.queue.pending} pending / ${snapshot.queue.running} running / ${snapshot.queue.retrying} retrying`,
		`Active runs: ${snapshot.runs.length}`,
	];

	if (orderedRuns.length === 0) {
		lines.push("- none");
	} else {
		lines.push(...orderedRuns.slice(0, MAX_SUMMARY_ITEMS).map(formatRun));
		if (orderedRuns.length > MAX_SUMMARY_ITEMS) {
			lines.push(`… ${orderedRuns.length - MAX_SUMMARY_ITEMS} more runs`);
		}
	}

	if (snapshot.agencies !== undefined) {
		lines.push(`Agencies: ${snapshot.agencies.length}`);
		lines.push(...snapshot.agencies.slice(0, MAX_SUMMARY_ITEMS).map(formatAgency));
		if (snapshot.agencies.length > MAX_SUMMARY_ITEMS) {
			lines.push(`… ${snapshot.agencies.length - MAX_SUMMARY_ITEMS} more agencies`);
		}
	}

	if (snapshot.budget) {
		lines.push(
			`Budget (${clean(snapshot.budget.run_id, 80)}): ${snapshot.budget.used}/${snapshot.budget.budget} used, ${snapshot.budget.remaining} remaining`,
		);
	}

	return lines.join("\n");
}
