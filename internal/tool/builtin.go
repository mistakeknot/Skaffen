package tool

// RegisterBuiltins adds all 7 built-in tools to the registry.
func RegisterBuiltins(r *Registry) {
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&EditTool{})
	r.Register(&BashTool{})
	r.Register(&GrepTool{})
	r.Register(&GlobTool{})
	r.Register(&LsTool{})
}
