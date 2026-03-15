package tool

// RegisterBuiltins adds all built-in tools to the registry.
func RegisterBuiltins(r *Registry) {
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&LsTool{})

	// Web tools — gated to brainstorm, plan, and build phases
	webPhases := []Phase{PhaseOrient, PhaseDecide, PhaseAct}
	r.RegisterForPhases(NewWebSearchTool(), webPhases)
	r.RegisterForPhases(NewWebFetchTool(), webPhases)
}

// RegisterQualityHistory adds the quality_history tool gated to Orient phase.
// Called separately because it requires a SignalReader from the mutations store.
func RegisterQualityHistory(r *Registry, store SignalReader) {
	r.RegisterForPhases(NewQualityHistoryTool(store), []Phase{PhaseOrient})
}
