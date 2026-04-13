// Package style provides mode-aware style fingerprinting — tracks how users
// communicate across different conversation modes and generates specific
// mirroring instructions.
//
// Port of Auraken's style.py. JSON wire-compatible with existing Python-generated
// fingerprints stored in core_profiles.style_fingerprint JSONB.
//
// Thread safety: ComputeObservables and ClassifyMode are pure functions (no locks).
// Fingerprint methods (Update, UpdateCadence, BuildMirroring) are mutex-protected
// and safe for concurrent use. DetectCurrentMode does not hold the Fingerprint
// mutex; the caller must ensure the messages slice is not concurrently modified.
package style
