import { readFileSync } from "node:fs";
import { describe, expect, it, vi } from "vitest";
import {
	findCurrentRun,
	inspectIntercore,
	parseIntercoreVersion,
	parseSituation,
	type IcRunner,
} from "../src/intercore.ts";
import type { ExecResult, IntercoreVersion, SituationSnapshot } from "../src/types.ts";

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

function version(overrides: Partial<IntercoreVersion> = {}): IntercoreVersion {
	return {
		version: "0.3.5",
		commit: "ac2dc66f59fc57a901c214080f99056a96de670f",
		commit_time: "2026-08-17T03:35:01Z",
		dirty: false,
		go: "go1.26.0",
		os: "darwin",
		arch: "arm64",
		source: "vcs",
		schema: 39,
		...overrides,
	};
}

function runnerFor(
	health: ExecResult,
	situation: ExecResult,
	versionResult: ExecResult = result({ stdout: JSON.stringify(version()) }),
): IcRunner {
	return vi.fn(async (args) => {
		if (args[0] === "health") return health;
		if (args[0] === "version") return versionResult;
		return situation;
	});
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
		expect(state.provenance).toBe("verified");
		expect(state.version).toEqual(version());
		expect(state.provenanceReason).toBeUndefined();
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

	it("reports unavailable for conventional command-not-found exit results", async () => {
		const missing = result({ code: 127, stderr: "ic: command not found\n" });
		const state = await inspectIntercore(runnerFor(missing, missing), CWD);

		expect(state.health).toBe("unavailable");
		expect(state.reason).toContain("command not found");
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

	it("propagates cancellation instead of converting it into ordinary state", async () => {
		const abort = Object.assign(new Error("cancelled"), { name: "AbortError" });
		const runner: IcRunner = vi.fn(async () => {
			throw abort;
		});

		await expect(inspectIntercore(runner, CWD)).rejects.toBe(abort);
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

	it("keeps operational health while surfacing a dirty build as unverified", async () => {
		const state = await inspectIntercore(
			runnerFor(
				result({ stdout: "ok\n" }),
				result({ stdout: JSON.stringify(snapshot()) }),
				result({ stdout: JSON.stringify(version({ dirty: true })) }),
			),
			CWD,
		);

		expect(state.health).toBe("healthy");
		expect(state.provenance).toBe("unverified");
		expect(state.provenanceReason).toContain("dirty");
	});

	it("surfaces timed-out provenance independently from operational health", async () => {
		const state = await inspectIntercore(
			runnerFor(
				result({ stdout: "ok\n" }),
				result({ stdout: JSON.stringify(snapshot()) }),
				result({ code: 143, killed: true }),
			),
			CWD,
		);

		expect(state.health).toBe("healthy");
		expect(state.provenance).toBe("unavailable");
		expect(state.provenanceReason).toContain("timed out");
	});

	it("surfaces clean but unstamped provenance as unverified", async () => {
		const state = await inspectIntercore(
			runnerFor(
				result({ stdout: "ok\n" }),
				result({ stdout: JSON.stringify(snapshot()) }),
				result({ stdout: JSON.stringify(version({ commit: undefined, source: "unknown" })) }),
			),
			CWD,
		);

		expect(state.health).toBe("healthy");
		expect(state.provenance).toBe("unverified");
		expect(state.provenanceReason).toContain("commit");
	});

	it("reports runtime schema independently from operational snapshot contents", async () => {
		const state = await inspectIntercore(
			runnerFor(
				result({ stdout: "ok\n" }),
				result({ stdout: JSON.stringify(snapshot()) }),
				result({ stdout: JSON.stringify(version({ schema: 39 })) }),
			),
			CWD,
		);

		expect(state.version?.schema).toBe(39);
		expect(state.snapshot).toEqual(snapshot());
	});

	it("keeps operational health while surfacing malformed provenance as unavailable", async () => {
		const state = await inspectIntercore(
			runnerFor(
				result({ stdout: "ok\n" }),
				result({ stdout: JSON.stringify(snapshot()) }),
				result({ stdout: "{not-json" }),
			),
			CWD,
		);

		expect(state.health).toBe("healthy");
		expect(state.provenance).toBe("unavailable");
		expect(state.provenanceReason).toContain("invalid ic version response");
		expect(state.version).toBeUndefined();
	});

	it("invokes only bounded read commands in the supplied cwd", async () => {
		const runner = runnerFor(result({ stdout: "ok\n" }), result({ stdout: JSON.stringify(snapshot()) }));

		await inspectIntercore(runner, CWD, 321);

		expect(runner).toHaveBeenCalledTimes(3);
		expect(runner).toHaveBeenCalledWith(["health", "--json"], { cwd: CWD, timeout: 321 });
		expect(runner).toHaveBeenCalledWith(["version", "--json"], { cwd: CWD, timeout: 321 });
		expect(runner).toHaveBeenCalledWith(["situation", "snapshot", "--json"], { cwd: CWD, timeout: 321 });
	});
});

describe("parseIntercoreVersion", () => {
	it("accepts the provenance-stamped schema-39 contract", () => {
		expect(parseIntercoreVersion(JSON.stringify(version()))).toEqual(version());
	});

	it.each([
		"",
		"null",
		"[]",
		JSON.stringify({ ...version(), dirty: "false" }),
		JSON.stringify({ ...version(), schema: 39.5 }),
		JSON.stringify({ ...version(), source: null }),
	])("rejects malformed version input %#", (input) => {
		expect(parseIntercoreVersion(input)).toBeUndefined();
	});
});

describe("parseSituation", () => {
	it.each(["situation-v37.json", "situation-v39.json"])(
		"accepts the pinned Intercore golden fixture %s",
		(filename) => {
			const fixture = readFileSync(new URL(`./fixtures/${filename}`, import.meta.url), "utf8");
			expect(parseSituation(fixture)).toEqual(JSON.parse(fixture));
		},
	);

	it("accepts schema-39 producer and agency ownership annotations", () => {
		const value = snapshot({
			runs: [
				{
					...snapshot().runs[0],
					producer: { kind: "agent_host", name: "pi", class: "interactive", version: "0.84.1" },
				},
			],
			agencies: [
				{
					name: "planning",
					cycle_id: "cycle-1",
					stage: "executing",
					event_type: "agency.stage.changed",
					run_id: "run-1",
					project_dir: CWD,
					last_event_id: 42,
					updated_at: "2026-08-18T19:45:23Z",
				},
			],
		});
		expect(parseSituation(JSON.stringify(value))).toEqual(value);
	});

	it("continues to accept a legacy schema-37 snapshot without ownership annotations", () => {
		expect(parseSituation(JSON.stringify(snapshot()))).toEqual(snapshot());
	});

	it.each([
		"",
		"null",
		"[]",
		JSON.stringify({ ...snapshot(), dispatches: { active: "zero", total: 0, agents: [] } }),
		JSON.stringify({ ...snapshot(), dispatches: { active: -1, total: 0, agents: [] } }),
		JSON.stringify({ ...snapshot(), dispatches: { active: 0, total: 0.5, agents: [] } }),
		JSON.stringify({ ...snapshot(), queue: { pending: -1, running: 0, retrying: 0 } }),
		JSON.stringify({ ...snapshot(), budget: { run_id: "run-1", budget: 10, used: 1.5, remaining: 8.5 } }),
		JSON.stringify({ ...snapshot(), runs: [{ id: "missing-fields" }] }),
		JSON.stringify({
			...snapshot(),
			runs: [{ ...snapshot().runs[0], producer: { kind: "agent_host" } }],
		}),
		JSON.stringify({
			...snapshot(),
			agencies: [
				{
					name: "planning",
					cycle_id: "cycle-1",
					stage: "executing",
					event_type: "agency.stage.changed",
					last_event_id: -1,
					updated_at: "2026-08-18T19:45:23Z",
				},
			],
		}),
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
