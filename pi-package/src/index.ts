import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { Box, Text } from "@earendil-works/pi-tui";
import { Type } from "typebox";
import { inspectIntercore } from "./intercore.ts";
import { formatCompactStatus, formatHarnessDetails, formatSituation } from "./presentation.ts";
import type { HarnessState } from "./types.ts";

const INTERCORE_TIMEOUT_MS = 250;
const STATUS_ENTRY = "skaffen-status";

export interface StatusEntryData {
	kind: "harness" | "situation";
	content: string;
	timestamp: string;
}

const ANSI_ESCAPE = /\u001b\[[0-?]*[ -/]*[@-~]/gu;
const UNSAFE_ENTRY_CONTROL = /[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/gu;
const MAX_ENTRY_CONTENT = 20_000;

function boundedEntryText(value: string, maxLength: number, multiline: boolean): string {
	const cleaned = value
		.replace(ANSI_ESCAPE, "")
		.replace(UNSAFE_ENTRY_CONTROL, "")
		.replace(/\r/gu, multiline ? "\n" : " ");
	const normalized = multiline ? cleaned : cleaned.replace(/\s+/gu, " ").trim();
	return normalized.slice(0, maxLength);
}

export function normalizeStatusEntryData(value: unknown): StatusEntryData {
	const record = typeof value === "object" && value !== null && !Array.isArray(value)
		? (value as Record<string, unknown>)
		: {};
	return {
		kind: record.kind === "situation" ? "situation" : "harness",
		content:
			typeof record.content === "string"
				? boundedEntryText(record.content, MAX_ENTRY_CONTENT, true)
				: "No Skaffen status data",
		timestamp:
			typeof record.timestamp === "string"
				? boundedEntryText(record.timestamp, 128, false)
				: new Date().toISOString(),
	};
}

function throwIfAborted(signal?: AbortSignal): void {
	if (!signal?.aborted) return;
	if (signal.reason instanceof Error) throw signal.reason;
	const error = new Error("Operation aborted");
	error.name = "AbortError";
	throw error;
}

function summarizeHarnessDetails(state: HarnessState): Record<string, unknown> {
	return {
		health: state.health,
		...(state.reason ? { reason: state.reason.slice(0, 240) } : {}),
		provenance: state.provenance,
		...(state.provenanceReason ? { provenanceReason: state.provenanceReason.slice(0, 240) } : {}),
		...(state.version
			? {
					version: {
						version: state.version.version.slice(0, 64),
						commit: state.version.commit?.slice(0, 64),
						dirty: state.version.dirty,
						source: state.version.source.slice(0, 64),
						schema: state.version.schema,
					},
				}
			: {}),
		checkedAt: state.checkedAt,
		latencyMs: state.latencyMs,
		...(state.currentRun
			? {
					currentRun: {
						id: state.currentRun.id.slice(0, 80),
						phase: state.currentRun.phase.slice(0, 48),
						oodarc_role: state.currentRun.oodarc_role?.slice(0, 48),
					},
				}
			: {}),
		...(state.snapshot
			? {
					snapshot: {
						timestamp: state.snapshot.timestamp.slice(0, 64),
						runCount: state.snapshot.runs.length,
						agencyCount: state.snapshot.agencies?.length ?? 0,
						dispatches: {
							active: state.snapshot.dispatches.active,
							total: state.snapshot.dispatches.total,
						},
						queue: state.snapshot.queue,
					},
				}
			: {}),
	};
}

function updateStatus(ctx: ExtensionContext, state: HarnessState): void {
	const color = state.health === "healthy" ? "success" : state.health === "degraded" ? "warning" : "error";
	ctx.ui.setStatus("skaffen", ctx.ui.theme.fg(color, formatCompactStatus(state)));
}

export default function skaffenExtension(pi: ExtensionAPI): void {
	let refreshGeneration = 0;

	async function refresh(ctx: ExtensionContext, signal?: AbortSignal): Promise<HarnessState> {
		const generation = ++refreshGeneration;
		throwIfAborted(signal);
		const state = await inspectIntercore(
			(args, options) =>
				pi.exec("ic", [...args], {
					...options,
					...(signal ? { signal } : {}),
				}),
			ctx.cwd,
			INTERCORE_TIMEOUT_MS,
		);
		throwIfAborted(signal);
		if (generation === refreshGeneration) updateStatus(ctx, state);
		return state;
	}

	function appendStatus(kind: StatusEntryData["kind"], content: string): void {
		pi.appendEntry<StatusEntryData>(STATUS_ENTRY, {
			kind,
			content,
			timestamp: new Date().toISOString(),
		});
	}

	pi.registerEntryRenderer<StatusEntryData>(STATUS_ENTRY, (entry, _options, theme) => {
		const data = normalizeStatusEntryData(entry.data);
		const box = new Box(1, 1, (text) => theme.bg("customMessageBg", text));
		box.addChild(
			new Text(`${theme.fg("accent", `[skaffen:${data.kind}]`)}\n${data.content}\n${theme.fg("dim", data.timestamp)}`, 0, 0),
		);
		return box;
	});

	pi.on("session_start", async (_event, ctx) => {
		await refresh(ctx);
	});

	pi.registerCommand("harness", {
		description: "Show Skaffen and Intercore harness health",
		handler: async (_args, ctx) => {
			const state = await refresh(ctx);
			const content = formatHarnessDetails(state);
			appendStatus("harness", content);
			ctx.ui.notify(`Skaffen harness: ${state.health}`, state.health === "healthy" ? "info" : "warning");
		},
	});

	pi.registerCommand("situation", {
		description: "Refresh and show the read-only Intercore situation snapshot",
		handler: async (_args, ctx) => {
			const state = await refresh(ctx);
			const content = formatSituation(state);
			appendStatus("situation", content);
			ctx.ui.notify(`Intercore situation: ${state.health}`, state.snapshot ? "info" : "warning");
		},
	});

	pi.registerTool({
		name: "observe_situation",
		label: "Observe Intercore situation",
		description:
			"Refresh and report Skaffen harness health plus the current read-only Intercore situation snapshot. This never initializes, migrates, or writes to Intercore.",
		promptSnippet: "Refresh read-only Intercore run, phase, OODARC, dispatch, and queue state.",
		promptGuidelines: [
			"Use observe_situation when operational run state may affect the next action; treat degraded or missing data as uncertainty, not an empty system.",
		],
		parameters: Type.Object({}, { additionalProperties: false }),
		executionMode: "parallel",
		execute: async (_toolCallId, _params, signal, _onUpdate, ctx) => {
			const state = await refresh(ctx, signal);
			return {
				content: [{ type: "text", text: formatSituation(state) }],
				details: summarizeHarnessDetails(state),
			};
		},
	});
}
