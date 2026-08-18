export interface ExecResult {
	stdout: string;
	stderr: string;
	code: number;
	killed: boolean;
}

export interface IntercoreVersion {
	version: string;
	commit?: string;
	commit_time?: string;
	dirty: boolean;
	go: string;
	os: string;
	arch: string;
	source: string;
	schema?: number;
}

export type IntercoreProvenance = "verified" | "unverified" | "unavailable";

export interface ProducerSummary {
	kind: string;
	name: string;
	class?: string;
	version?: string;
}

export interface AgencySummary {
	name: string;
	cycle_id: string;
	stage: string;
	event_type: string;
	run_id?: string;
	project_dir?: string;
	last_event_id: number;
	updated_at: string;
}

export interface RunSummary {
	id: string;
	phase: string;
	oodarc_role?: string;
	status: string;
	project_dir: string;
	goal: string;
	created_at: number;
	producer?: ProducerSummary;
}

export interface DispatchSummary {
	active: number;
	total: number;
	agents: unknown[];
}

export interface QueueSummary {
	pending: number;
	running: number;
	retrying: number;
}

export interface BudgetSummary {
	run_id: string;
	budget: number;
	used: number;
	remaining: number;
}

export interface SituationSnapshot {
	timestamp: string;
	runs: RunSummary[];
	dispatches: DispatchSummary;
	recent_events: unknown[];
	agencies?: AgencySummary[];
	queue: QueueSummary;
	budget?: BudgetSummary;
}

export type IntercoreHealth = "healthy" | "degraded" | "unavailable";

export interface HarnessState {
	health: IntercoreHealth;
	reason?: string;
	provenance: IntercoreProvenance;
	provenanceReason?: string;
	version?: IntercoreVersion;
	checkedAt: string;
	latencyMs: number;
	snapshot?: SituationSnapshot;
	currentRun?: RunSummary;
}
