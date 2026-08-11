import type {
	ExtensionAPI,
	ExtensionCommandContext,
	ExtensionContext,
	RegisteredCommand,
	ToolDefinition,
} from "@earendil-works/pi-coding-agent";
import { beforeEach, describe, expect, it, vi } from "vitest";
import skaffenExtension from "../src/index.ts";
import type { ExecResult, SituationSnapshot } from "../src/types.ts";

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

function createHarness() {
	const handlers = new Map<string, EventHandler>();
	const commands = new Map<string, Registered>();
	const tools = new Map<string, ToolDefinition>();
	const statuses: Array<{ key: string; text: string | undefined }> = [];
	const entries: Array<{ customType: string; data: unknown }> = [];
	const notifications: string[] = [];

	const exec = vi.fn(
		async (_command: string, args: string[]): Promise<ExecResult> =>
			args[0] === "health"
				? execResult({
						code: 2,
						stderr: `${JSON.stringify({ error: "schema version 30 is older than current 37" })}\n`,
					})
				: execResult({ stdout: JSON.stringify(snapshot()) }),
	);

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

		expect(harness.exec).toHaveBeenCalledTimes(2);
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["health", "--json"],
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

		expect(harness.exec).toHaveBeenCalledTimes(4);
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

	it("returns the same refreshed situation to the agent without mutation", async () => {
		const tool = harness.tools.get("observe_situation");
		const signal = new AbortController().signal;
		const output = await tool?.execute("call-1", {}, signal, undefined, harness.ctx);

		expect(output?.content).toEqual([
			expect.objectContaining({ type: "text", text: expect.stringContaining("run-1") }),
		]);
		expect(output?.details).toEqual(expect.objectContaining({ health: "degraded" }));
		expect(harness.exec).toHaveBeenCalledWith(
			"ic",
			["health", "--json"],
			expect.objectContaining({ signal }),
		);
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
				["situation", "snapshot", "--json"],
			]),
		);
		expect(argumentVectors.every((args) => args[0] === "health" || args[0] === "situation")).toBe(true);
	});
});
