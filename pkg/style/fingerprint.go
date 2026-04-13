package style

// ema computes an exponential moving average.
// Exact operator form preserved from Python: old*(1-alpha) + new*alpha.
// Do NOT refactor to old + alpha*(new-old) — floating-point non-associativity
// causes compound divergence for converging values.
func ema(old, new, alpha float64) float64 {
	return old*(1-alpha) + new*alpha
}

// computeAlpha returns the EMA smoothing factor.
// Uses n's value BEFORE increment: alpha = 0.3 when n >= 5, else 1/(n+1).
// At the 6th message (n=5 going in), alpha switches to 0.3.
func computeAlpha(n int) float64 {
	if n >= 5 {
		return 0.3
	}
	return 1.0 / float64(n+1)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// update applies new observations to a mode profile. Must be called under lock.
func (p *ModeProfile) update(obs Observables) {
	alpha := computeAlpha(p.N)

	p.AvgWords = ema(p.AvgWords, float64(obs.WordCount), alpha)
	p.CapitalizationRatio = ema(p.CapitalizationRatio, obs.CapitalizationRatio, alpha)
	p.EmojiDensity = ema(p.EmojiDensity, obs.EmojiDensity, alpha)

	p.PctContraction = ema(p.PctContraction, boolToFloat(obs.HasContraction), alpha)
	p.PctQuestion = ema(p.PctQuestion, boolToFloat(obs.HasQuestion), alpha)
	p.PctPeriod = ema(p.PctPeriod, boolToFloat(obs.HasPeriod), alpha)
	p.PctExclamation = ema(p.PctExclamation, boolToFloat(obs.HasExclamation), alpha)
	p.PctLowercase = ema(p.PctLowercase, boolToFloat(obs.IsAllLowercase), alpha)
	p.PctMultiSentence = ema(p.PctMultiSentence, boolToFloat(obs.IsMultiSentence), alpha)

	// Vocabulary counters — raw increment, not EMA.
	for _, token := range obs.Laughter {
		p.Laughter[token]++
	}
	for _, token := range obs.Affirmation {
		p.Affirmation[token]++
	}
	// Note: Observables uses plural "Intensifiers", ModeProfile uses singular "Intensifier".
	for _, token := range obs.Intensifiers {
		p.Intensifier[token]++
	}
	for _, token := range obs.Hedges {
		p.Hedge[token]++
	}

	if obs.Opener != "" {
		p.Opener[obs.Opener]++
	}

	p.N++
}

// Update applies new observations to the fingerprint. Thread-safe.
// Skips zero-value Observables (matching Python's `if not new_obs: return existing`).
func (f *Fingerprint) Update(obs Observables) {
	if obs.WordCount == 0 && obs.Mode == "" {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.Global.update(obs)

	mode := obs.Mode
	if mode == "" {
		mode = ModeGeneral
	}
	profile, ok := f.Modes[mode]
	if !ok {
		profile = NewModeProfile()
		f.Modes[mode] = profile
	}
	profile.update(obs)

	f.MessageCount++
}

// UpdateCadence updates cadence tracking with a new burst observation.
// Uses the SAME mutex as Update — all Fingerprint mutations share one lock.
func (f *Fingerprint) UpdateCadence(burstSize int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.updateCadenceInternal(burstSize)
}

func (f *Fingerprint) updateCadenceInternal(burstSize int) {
	alpha := computeAlpha(f.Cadence.BurstCount)
	f.Cadence.AvgBurstSize = ema(f.Cadence.AvgBurstSize, float64(burstSize), alpha)
	f.Cadence.BurstCount++
}

// UpdateWithCadence atomically applies observations and cadence in a single lock.
func (f *Fingerprint) UpdateWithCadence(obs Observables, burstSize int) {
	if obs.WordCount == 0 && obs.Mode == "" {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.Global.update(obs)

	mode := obs.Mode
	if mode == "" {
		mode = ModeGeneral
	}
	profile, ok := f.Modes[mode]
	if !ok {
		profile = NewModeProfile()
		f.Modes[mode] = profile
	}
	profile.update(obs)

	f.MessageCount++
	f.updateCadenceInternal(burstSize)
}
