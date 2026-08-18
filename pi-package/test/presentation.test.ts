import { describe, expect, it } from "vitest";
import { formatCompactStatus, formatHarnessDetails, formatSituation } from "../src/presentation.ts";
import type { AgencySummary, HarnessState, RunSummary, SituationSnapshot } from "../src/types.ts";

const currentRun: RunSummary = {
	id: "run-current",
	phase: "plan",
	oodarc_role: "decide",
	status: "active",
	project_dir: "/repo",
	goal: "build the Pi adapter",
	created_at: 1_786_468_000,
	producer: { kind: "agent_host", name: "pi", class: "interactive", version: "0.84.1" },
};

const otherRun: RunSummary = {
	...currentRun,
	id: "run-other",
	phase: "brainstorm",
	oodarc_role: "observe",
	project_dir: "/other",
	goal: "unrelated work",
};

function snapshot(runs: RunSummary[] = [currentRun, otherRun]): SituationSnapshot {
	return {
		timestamp: "2026-08-11T17:19:48.031953Z",
		runs,
		dispatches: { active: 2, total: 3, agents: [] },
		recent_events: [],
		queue: { pending: 4, running: 1, retrying: 2 },
	};
}

function state(overrides: Partial<HarnessState> = {}): HarnessState {
	return {
		health: "healthy",
		provenance: "verified",
		version: {
			version: "0.3.5",
			commit: "ac2dc66f59fc57a901c214080f99056a96de670f",
			commit_time: "2026-08-17T03:35:01Z",
			dirty: false,
			go: "go1.26.0",
			os: "darwin",
			arch: "arm64",
			source: "vcs",
			schema: 39,
		},
		checkedAt: "2026-08-11T17:19:48.050Z",
		latencyMs: 41,
		snapshot: snapshot(),
		currentRun,
		...overrides,
	};
}

describe("formatCompactStatus", () => {
	it("shows phase and OODARC role for the current run", () => {
		expect(formatCompactStatus(state())).toBe("◆ plan · decide");
	});

	it("shows global activity without claiming an unrelated run is current", () => {
		expect(formatCompactStatus(state({ currentRun: undefined }))).toBe("◆ idle · 2 active");
	});

	it("marks a readable but unhealthy Intercore as degraded", () => {
		expect(formatCompactStatus(state({ health: "degraded", reason: "schema mismatch" }))).toBe(
			"◇! plan · decide",
		);
	});

	it("shows unavailable Intercore without stale run state", () => {
		expect(
			formatCompactStatus(
				state({ health: "unavailable", reason: "ic unavailable", snapshot: undefined, currentRun: undefined }),
			),
		).toBe("◇ ic offline");
	});

	it("omits an unknown OODARC role rather than inventing one", () => {
		expect(formatCompactStatus(state({ currentRun: { ...currentRun, oodarc_role: undefined } }))).toBe("◆ plan");
	});

	it("bounds untrusted phase and role text", () => {
		const status = formatCompactStatus(
			state({ currentRun: { ...currentRun, phase: "x".repeat(200), oodarc_role: "y".repeat(200) } }),
		);
		expect(status.length).toBeLessThanOrEqual(80);
	});
});

describe("formatHarnessDetails", () => {
	it("includes health, freshness, latency, and current run", () => {
		const output = formatHarnessDetails(state());
		expect(output).toContain("Health: healthy");
		expect(output).toContain("Checked: 2026-08-11T17:19:48.050Z");
		expect(output).toContain("Latency: 41 ms");
		expect(output).toContain("Intercore: 0.3.5 · schema 39");
		expect(output).toContain("Build: ac2dc66f59fc · verified · vcs");
		expect(output).toContain("Current run: run-current");
		expect(output).toContain("Phase: plan (decide)");
		expect(output).toContain("Producer: agent_host/pi@0.84.1 (interactive)");
	});

	it("surfaces unavailable provenance without changing operational health", () => {
		const output = formatHarnessDetails(
			state({
				provenance: "unavailable",
				provenanceReason: "invalid ic version response",
				version: undefined,
			}),
		);
		expect(output).toContain("Health: healthy");
		expect(output).toContain("Provenance: unavailable — invalid ic version response");
	});

	it("sanitizes and bounds external diagnostics", () => {
		const output = formatHarnessDetails(
			state({ health: "degraded", reason: `bad\u001b[31m\n${"x".repeat(500)}` }),
		);
		expect(output).not.toContain("\u001b");
		expect(output).not.toContain("\nxxx");
		const reasonLine = output.split("\n").find((line) => line.startsWith("Reason:"));
		expect(reasonLine?.length).toBeLessThanOrEqual(250);
	});
});

describe("formatSituation", () => {
	it("renders the current run before unrelated active runs", () => {
		const output = formatSituation(state());
		expect(output.indexOf("run-current")).toBeLessThan(output.indexOf("run-other"));
	});

	it("renders producer and agency ownership annotations without inferring authority", () => {
		const agency: AgencySummary = {
			name: "planning",
			cycle_id: "cycle-1",
			stage: "executing",
			event_type: "agency.stage.changed",
			run_id: currentRun.id,
			project_dir: currentRun.project_dir,
			last_event_id: 42,
			updated_at: "2026-08-18T19:45:23Z",
		};
		const output = formatSituation(state({ snapshot: { ...snapshot(), agencies: [agency] } }));
		expect(output).toContain("producer: agent_host/pi@0.84.1");
		expect(output).toContain("Agencies: 1");
		expect(output).toContain("planning/cycle-1: executing · agency.stage.changed");
		expect(output).not.toContain("authorized");
	});

	it("bounds run and agency collections", () => {
		const runs = Array.from({ length: 25 }, (_, index) => ({
			...currentRun,
			id: `run-${index}`,
			project_dir: `/repo/${index}`,
		}));
		const agencies = Array.from({ length: 25 }, (_, index): AgencySummary => ({
			name: `agency-${index}`,
			cycle_id: `cycle-${index}`,
			stage: "executing",
			event_type: "agency.stage.changed",
			last_event_id: index,
			updated_at: "2026-08-18T19:45:23Z",
		}));
		const output = formatSituation(
			state({ currentRun: runs[0], snapshot: { ...snapshot(runs), agencies } }),
		);
		expect(output).toContain("… 5 more runs");
		expect(output).toContain("… 5 more agencies");
		expect(output).not.toContain("run-24:");
		expect(output).not.toContain("agency-24/");
	});

	it("includes dispatch and queue counts", () => {
		const output = formatSituation(state());
		expect(output).toContain("Dispatches: 2 active / 3 total");
		expect(output).toContain("Queue: 4 pending / 1 running / 2 retrying");
	});

	it("explains when no snapshot is available", () => {
		const output = formatSituation(
			state({ health: "unavailable", reason: "ic unavailable", snapshot: undefined, currentRun: undefined }),
		);
		expect(output).toContain("Situation snapshot unavailable");
		expect(output).toContain("ic unavailable");
	});
});
