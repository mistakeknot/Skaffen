package tool

// RegisterBuiltins adds all 9 built-in tools to the registry.
func RegisterBuiltins(r *Registry) {
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&LsTool{})

	// Web tools — gated to brainstorm, plan, and build phases
	webPhases := []Phase{PhaseBrainstorm, PhasePlan, PhaseBuild}
	r.RegisterForPhases(NewWebSearchTool(), webPhases)
	r.RegisterForPhases(NewWebFetchTool(), webPhases)
}
