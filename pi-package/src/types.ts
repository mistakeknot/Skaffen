export interface ExecResult {
	stdout: string;
	stderr: string;
	code: number;
	killed: boolean;
}

export interface RunSummary {
	id: string;
	phase: string;
	oodarc_role?: string;
	status: string;
	project_dir: string;
	goal: string;
	created_at: number;
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
	queue: QueueSummary;
	budget?: BudgetSummary;
}

export type IntercoreHealth = "healthy" | "degraded" | "unavailable";

export interface HarnessState {
	health: IntercoreHealth;
	reason?: string;
	checkedAt: string;
	latencyMs: number;
	snapshot?: SituationSnapshot;
	currentRun?: RunSummary;
}
