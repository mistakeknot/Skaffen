import { describe, expect, it, vi } from "vitest";
import {
	findCurrentRun,
	inspectIntercore,
	parseSituation,
	type IcRunner,
} from "../src/intercore.ts";
import type { ExecResult, SituationSnapshot } from "../src/types.ts";

const CWD = "/repo/packages/app";

function result(overrides: Partial<ExecResult> = {}): ExecResult {
	return { stdout: "", stderr: "", code: 0, killed: false, ...overrides };
}

function snapshot(overrides: Partial<SituationSnapshot> = {}): SituationSnapshot {
	return {
		timestamp: "2026-08-11T17:19:48.031953Z",
		runs: [
			{
				id: "run-1",
				phase: "plan",
				oodarc_role: "decide",
				status: "active",
				project_dir: CWD,
				goal: "ship the adapter",
				created_at: 1_786_468_000,
			},
		],
		dispatches: { active: 0, total: 0, agents: [] },
		recent_events: [],
		queue: { pending: 0, running: 0, retrying: 0 },
		...overrides,
	};
}

function runnerFor(health: ExecResult, situation: ExecResult): IcRunner {
	return vi.fn(async (args) => (args[0] === "health" ? health : situation));
}

describe("inspectIntercore", () => {
	it("reports healthy state with a valid snapshot and matching run", async () => {
		const snap = snapshot();
		const state = await inspectIntercore(
			runnerFor(result({ stdout: "ok\n" }), result({ stdout: JSON.stringify(snap) })),
			CWD,
		);

		expect(state.health).toBe("healthy");
		expect(state.reason).toBeUndefined();
		expect(state.snapshot).toEqual(snap);
		expect(state.currentRun?.id).toBe("run-1");
		expect(state.checkedAt).toMatch(/^\d{4}-\d{2}-\d{2}T/);
		expect(state.latencyMs).toBeGreaterThanOrEqual(0);
	});

	it("retains a readable snapshot when health reports schema mismatch", async () => {
		const healthError = JSON.stringify({
			level: "ERROR",
			msg: "health failed",
			error: "health: schema version 30 is older than current 37: database not migrated — run 'ic init'",
		});
		const state = await inspectIntercore(
			runnerFor(result({ code: 2, stderr: `${healthError}\n` }), result({ stdout: JSON.stringify(snapshot()) })),
			CWD,
		);

		expect(state.health).toBe("degraded");
		expect(state.reason).toContain("schema version 30 is older than current 37");
		expect(state.snapshot?.runs).toHaveLength(1);
	});

	it("reports unavailable when neither command can execute", async () => {
		const state = await inspectIntercore(
			runnerFor(result({ code: 1 }), result({ code: 1 })),
			CWD,
		);

		expect(state.health).toBe("unavailable");
		expect(state.reason).toContain("unavailable");
		expect(state.snapshot).toBeUndefined();
	});

	it("never reports healthy when a command times out", async () => {
		const state = await inspectIntercore(
			runnerFor(result({ code: 143, killed: true }), result({ stdout: JSON.stringify(snapshot()) })),
			CWD,
		);

		expect(state.health).toBe("degraded");
		expect(state.reason).toContain("timed out");
		expect(state.snapshot).toBeDefined();
	});

	it("degrades when situation output is malformed JSON", async () => {
		const state = await inspectIntercore(
			runnerFor(result({ stdout: "ok\n" }), result({ stdout: "{not-json" })),
			CWD,
		);

		expect(state.health).toBe("degraded");
		expect(state.reason).toContain("invalid situation snapshot");
		expect(state.snapshot).toBeUndefined();
	});

	it("degrades when situation JSON has the wrong shape", async () => {
		const state = await inspectIntercore(
			runnerFor(result({ stdout: "ok\n" }), result({ stdout: JSON.stringify({ timestamp: "now", runs: {} }) })),
			CWD,
		);

		expect(state.health).toBe("degraded");
		expect(state.reason).toContain("invalid situation snapshot");
	});

	it("contains runner rejections instead of throwing", async () => {
		const runner: IcRunner = vi.fn(async () => {
			throw new Error("spawn ic ENOENT");
		});

		await expect(inspectIntercore(runner, CWD)).resolves.toMatchObject({
			health: "unavailable",
			reason: expect.stringContaining("spawn ic ENOENT"),
		});
	});

	it("invokes only bounded read commands in the supplied cwd", async () => {
		const runner = runnerFor(result({ stdout: "ok\n" }), result({ stdout: JSON.stringify(snapshot()) }));

		await inspectIntercore(runner, CWD, 321);

		expect(runner).toHaveBeenCalledTimes(2);
		expect(runner).toHaveBeenCalledWith(["health", "--json"], { cwd: CWD, timeout: 321 });
		expect(runner).toHaveBeenCalledWith(["situation", "snapshot", "--json"], { cwd: CWD, timeout: 321 });
	});
});

describe("parseSituation", () => {
	it("accepts the documented snapshot shape", () => {
		expect(parseSituation(JSON.stringify(snapshot()))).toEqual(snapshot());
	});

	it.each([
		"",
		"null",
		"[]",
		JSON.stringify({ ...snapshot(), dispatches: { active: "zero", total: 0, agents: [] } }),
		JSON.stringify({ ...snapshot(), runs: [{ id: "missing-fields" }] }),
	])("rejects malformed snapshot input %#", (input) => {
		expect(parseSituation(input)).toBeUndefined();
	});
});

describe("findCurrentRun", () => {
	it("chooses the deepest project directory enclosing cwd", () => {
		const snap = snapshot({
			runs: [
				{ ...snapshot().runs[0], id: "root", project_dir: "/repo" },
				{ ...snapshot().runs[0], id: "app", project_dir: "/repo/packages/app" },
				{ ...snapshot().runs[0], id: "other", project_dir: "/other" },
			],
		});

		expect(findCurrentRun(snap, "/repo/packages/app/src")?.id).toBe("app");
	});

	it("does not present an unrelated active run as current", () => {
		const snap = snapshot({ runs: [{ ...snapshot().runs[0], project_dir: "/other" }] });
		expect(findCurrentRun(snap, CWD)).toBeUndefined();
	});
});
