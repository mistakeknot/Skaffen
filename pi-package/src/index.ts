import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { Box, Text } from "@earendil-works/pi-tui";
import { Type } from "typebox";
import { inspectIntercore } from "./intercore.ts";
import { formatCompactStatus, formatHarnessDetails, formatSituation } from "./presentation.ts";
import type { HarnessState } from "./types.ts";

const INTERCORE_TIMEOUT_MS = 250;
const STATUS_ENTRY = "skaffen-status";

interface StatusEntryData {
	kind: "harness" | "situation";
	content: string;
	timestamp: string;
}

function updateStatus(ctx: ExtensionContext, state: HarnessState): void {
	const color = state.health === "healthy" ? "success" : state.health === "degraded" ? "warning" : "error";
	ctx.ui.setStatus("skaffen", ctx.ui.theme.fg(color, formatCompactStatus(state)));
}

export default function skaffenExtension(pi: ExtensionAPI): void {
	async function refresh(ctx: ExtensionContext, signal?: AbortSignal): Promise<HarnessState> {
		const state = await inspectIntercore(
			(args, options) =>
				pi.exec("ic", [...args], {
					...options,
					...(signal ? { signal } : {}),
				}),
			ctx.cwd,
			INTERCORE_TIMEOUT_MS,
		);
		updateStatus(ctx, state);
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
		const data = entry.data ?? {
			kind: "harness",
			content: "No Skaffen status data",
			timestamp: new Date().toISOString(),
		};
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
				details: state,
			};
		},
	});
}
