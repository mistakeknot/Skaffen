//! OODARC coupling spike — v0.1 no-op hook.
//!
//! Defines the OodarcHook trait with phase lifecycle methods.
//! v0.1 wires a NoopOodarcHook at the agent turn boundary to
//! prove the insertion point works. v0.2 replaces with the
//! full OODARC state machine.
//!
//! Gated behind the `skaffen-oodarc` feature flag.

/// OODARC phase lifecycle hook.
///
/// Called at the agent turn boundary to allow phase-aware
/// processing. Each method corresponds to one OODARC phase.
pub trait OodarcHook: Send + Sync {
    fn on_observe(&self) {}
    fn on_orient(&self) {}
    fn on_decide(&self) {}
    fn on_act(&self) {}
    fn on_reflect(&self) {}
    fn on_compound(&self) {}
}

/// No-op implementation for v0.1 coupling spike.
pub struct NoopOodarcHook;

impl OodarcHook for NoopOodarcHook {}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noop_hook_compiles_and_runs() {
        let hook = NoopOodarcHook;
        hook.on_observe();
        hook.on_orient();
        hook.on_decide();
        hook.on_act();
        hook.on_reflect();
        hook.on_compound();
    }
}
