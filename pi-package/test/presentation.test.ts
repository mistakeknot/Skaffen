import { describe, expect, it } from "vitest";
import { formatCompactStatus, formatHarnessDetails, formatSituation } from "../src/presentation.ts";
import type { HarnessState, RunSummary, SituationSnapshot } from "../src/types.ts";

const currentRun: RunSummary = {
	id: "run-current",
	phase: "plan",
	oodarc_role: "decide",
	status: "active",
	project_dir: "/repo",
	goal: "build the Pi adapter",
	created_at: 1_786_468_000,
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
		expect(output).toContain("Current run: run-current");
		expect(output).toContain("Phase: plan (decide)");
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
