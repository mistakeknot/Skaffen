import type {
	ExtensionAPI,
	ExtensionCommandContext,
	ExtensionContext,
	RegisteredCommand,
	ToolDefinition,
} from "@earendil-works/pi-coding-agent";
import { beforeEach, describe, expect, it, vi } from "vitest";
import skaffenExtension, { normalizeStatusEntryData } from "../src/index.ts";
import type { ExecResult, IntercoreVersion, SituationSnapshot } from "../src/types.ts";

type EventHandler = (event: Record<string, unknown>, ctx: ExtensionContext) => Promise<unknown> | unknown;

type Registered = Omit<RegisteredCommand, "name" | "sourceInfo">;

const CWD = "/repo";

function snapshot(): SituationSnapshot {
	return {
		timestamp: "2026-08-11T17:19:48.031953Z",
		runs: [
			{
				id: "run-1",
				phase: "plan",
				oodarc_role: "decide",
				status: "active",
				project_dir: CWD,
				goal: "build adapter",
				created_at: 1_786_468_000,
			},
		],
		dispatches: { active: 0, total: 0, agents: [] },
		recent_events: [],
		queue: { pending: 0, running: 0, retrying: 0 },
	};
}

function execResult(overrides: Partial<ExecResult> = {}): ExecResult {
	return { stdout: "", stderr: "", code: 0, killed: false, ...overrides };
}

function version(): IntercoreVersion {
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
	};
}

function createHarness() {
	const handlers = new Map<string, EventHandler>();
	const commands = new Map<string, Registered>();
	const tools = new Map<string, ToolDefinition>();
	const statuses: Array<{ key: string; text: string | undefined }> = [];
	const entries: Array<{ customType: string; data: unknown }> = [];
	const notifications: string[] = [];

	const exec = vi.fn(async (_command: string, args: string[]): Promise<ExecResult> => {
		if (args[0] === "health") {
			return execResult({
				code: 2,
				stderr: `${JSON.stringify({ error: "schema version 30 is older than current 37" })}\n`,
			});
		}
		if (args[0] === "version") return execResult({ stdout: JSON.stringify(version()) });
		return execResult({ stdout: JSON.stringify(snapshot()) });
	});

	const pi = {
		on: vi.fn((event: string, handler: EventHandler) => handlers.set(event, handler)),
		registerCommand: vi.fn((name: string, command: Registered) => commands.set(name, command)),
		registerTool: vi.fn((tool: ToolDefinition) => tools.set(tool.name, tool)),
		registerEntryRenderer: vi.fn(),
		appendEntry: vi.fn((customType: string, data: unknown) => entries.push({ customType, data })),
		exec,
	} as unknown as ExtensionAPI;

	const ctx = {
		cwd: CWD,
		hasUI: true,
		mode: "tui",
		ui: {
			theme: { fg: (_color: string, text: string) => text },
			setStatus: (key: string, text: string | undefined) => statuses.push({ key, text }),
			notify: (message: string) => notifications.push(message),
		},
	} as unknown as ExtensionContext & ExtensionCommandContext;

	return { pi, ctx, handlers, commands, tools, statuses, entries, notifications, exec };
}

describe("normalizeStatusEntryData", () => {
	it("bounds and sanitizes persisted entry data", () => {
		const normalized = normalizeStatusEntryData({
			kind: "unexpected",
			content: `hello\u001b[31m\u0000\n${"x".repeat(25_000)}`,
			timestamp: "2026-08-18T19:45:23Z\nforged",
		});
		expect(normalized.kind).toBe("harness");
		expect(normalized.content).not.toContain("\u001b");
		expect(normalized.content).not.toContain("\u0000");
		expect(normalized.content.length).toBeLessThanOrEqual(20_000);
		expect(normalized.timestamp).toBe("2026-08-18T19:45:23Z forged");
	});
});

describe("Skaffen Pi extension", () => {
	let harness: ReturnType<typeof createHarness>;

	beforeEach(() => {
		harness = createHarness();
		skaffenExtension(harness.pi);
	});

	it("registers the session lifecycle, human commands, and agent observation tool", () => {
		expect(harness.handlers.has("session_start")).toBe(true);
		expect([...harness.commands.keys()]).toEqual(["harness", "situation"]);
		expect(harness.tools.has("observe_situation")).toBe(true);
		expect(harness.pi.registerEntryRenderer).toHaveBeenCalledWith("skaffen-status", expect.any(Function));
	});

	it("refreshes bounded read-only state on session start and updates status", async () => {
		await harness.handlers.get("session_start")?.({ type: "session_start", reason: "startup" }, harness.ctx);

		expect(harness.exec).toHaveBeenCalledTimes(3);
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["health", "--json"],
			expect.objectContaining({ cwd: CWD, timeout: 250 }),
		);
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["version", "--json"],
			expect.objectContaining({ cwd: CWD, timeout: 250 }),
		);
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["situation", "snapshot", "--json"],
			expect.objectContaining({ cwd: CWD, timeout: 250 }),
		);
		expect(harness.statuses.at(-1)).toEqual({ key: "skaffen", text: "◇! plan · decide" });
	});

	it("renders durable command cards and refreshes on every invocation", async () => {
		await expect(harness.commands.get("harness")?.handler("", harness.ctx)).resolves.toBeUndefined();
		await expect(harness.commands.get("situation")?.handler("", harness.ctx)).resolves.toBeUndefined();

		expect(harness.exec).toHaveBeenCalledTimes(6);
		expect(harness.entries.map((entry) => entry.customType)).toEqual(["skaffen-status", "skaffen-status"]);
		expect(harness.entries[0]?.data).toEqual(
			expect.objectContaining({ kind: "harness", content: expect.stringContaining("Health: degraded") }),
		);
		expect(harness.entries[1]?.data).toEqual(
			expect.objectContaining({ kind: "situation", content: expect.stringContaining("run-1") }),
		);
		expect(harness.notifications).toEqual(["Skaffen harness: degraded", "Intercore situation: degraded"]);
		expect(harness.notifications.every((message) => !message.includes("\n"))).toBe(true);
	});

	it("honors a pre-aborted observation without invoking Intercore", async () => {
		const tool = harness.tools.get("observe_situation");
		const controller = new AbortController();
		controller.abort(new Error("cancelled"));

		await expect(tool?.execute("call-1", {}, controller.signal, undefined, harness.ctx)).rejects.toThrow(
			"cancelled",
		);
		expect(harness.exec).not.toHaveBeenCalled();
	});

	it("returns a bounded refreshed situation summary to the agent without mutation", async () => {
		const tool = harness.tools.get("observe_situation");
		const signal = new AbortController().signal;
		const output = await tool?.execute("call-1", {}, signal, undefined, harness.ctx);

		expect(output?.content).toEqual([
			expect.objectContaining({ type: "text", text: expect.stringContaining("run-1") }),
		]);
		expect(output?.details).toEqual(
			expect.objectContaining({
				health: "degraded",
				provenance: "verified",
				snapshot: expect.objectContaining({ runCount: 1, agencyCount: 0 }),
			}),
		);
		expect((output?.details as { snapshot?: Record<string, unknown> }).snapshot).not.toHaveProperty("runs");
		expect((output?.details as { snapshot?: Record<string, unknown> }).snapshot).not.toHaveProperty("recent_events");
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["health", "--json"],
			expect.objectContaining({ signal }),
		);
	});

	it("does not let an older overlapping refresh overwrite newer footer state", async () => {
		let callIndex = 0;
		let releaseFirst: (() => void) | undefined;
		const firstGate = new Promise<void>((resolve) => {
			releaseFirst = resolve;
		});
		harness.exec.mockImplementation(async (_command: string, args: string[]) => {
			const generation = Math.floor(callIndex++ / 3);
			if (generation === 0) await firstGate;
			if (args[0] === "health") {
				return generation === 0
					? execResult({ code: 2, stderr: JSON.stringify({ error: "old degraded state" }) })
					: execResult({ stdout: "ok\n" });
			}
			if (args[0] === "version") return execResult({ stdout: JSON.stringify(version()) });
			return execResult({ stdout: JSON.stringify(snapshot()) });
		});

		const first = harness.commands.get("harness")?.handler("", harness.ctx);
		const second = harness.commands.get("harness")?.handler("", harness.ctx);
		await second;
		releaseFirst?.();
		await first;

		expect(harness.statuses.at(-1)).toEqual({ key: "skaffen", text: "◆ plan · decide" });
	});

	it("never invokes an Intercore write command", async () => {
		await harness.handlers.get("session_start")?.({ type: "session_start", reason: "startup" }, harness.ctx);
		await harness.commands.get("harness")?.handler("", harness.ctx);
		await harness.commands.get("situation")?.handler("", harness.ctx);
		await harness.tools.get("observe_situation")?.execute("call-1", {}, undefined, undefined, harness.ctx);

		const argumentVectors = harness.exec.mock.calls.map((call) => call[1]);
		expect(argumentVectors).toEqual(
			expect.arrayContaining([
				["health", "--json"],
				["version", "--json"],
				["situation", "snapshot", "--json"],
			]),
		);
		const uniqueVectors = [...new Set(argumentVectors.map((args) => JSON.stringify(args)))].sort();
		expect(uniqueVectors).toEqual(
			[["health", "--json"], ["situation", "snapshot", "--json"], ["version", "--json"]]
				.map((args) => JSON.stringify(args))
				.sort(),
		);
	});
});
