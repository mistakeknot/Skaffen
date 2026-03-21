package router

import (
	"runtime"
	"testing"
)

func TestDetectHardware(t *testing.T) {
	p := DetectHardware()

	if p.CPUCores != runtime.NumCPU() {
		t.Errorf("CPUCores = %d, want %d", p.CPUCores, runtime.NumCPU())
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if p.MemoryMB == 0 {
			t.Error("MemoryMB = 0 on supported OS")
		}
	}
	// Tier should be valid
	if p.Tier < TierConstrained || p.Tier > TierHigh {
		t.Errorf("Tier = %d, want 0-3", p.Tier)
	}
}

func TestClassifyTier(t *testing.T) {
	tests := []struct {
		name    string
		profile HardwareProfile
		want    HardwareTier
	}{
		{"constrained-low-memory", HardwareProfile{CPUCores: 2, MemoryMB: 2048}, TierConstrained},
		{"constrained-low-cpu", HardwareProfile{CPUCores: 1, MemoryMB: 8192}, TierConstrained},
		{"standard", HardwareProfile{CPUCores: 4, MemoryMB: 8192}, TierStandard},
		{"capable-memory", HardwareProfile{CPUCores: 4, MemoryMB: 32768}, TierCapable},
		{"capable-cores", HardwareProfile{CPUCores: 12, MemoryMB: 8192}, TierCapable},
		{"high-memory", HardwareProfile{CPUCores: 4, MemoryMB: 131072}, TierHigh},
		{"high-gpu", HardwareProfile{CPUCores: 4, MemoryMB: 8192, GPUMemoryMB: 16384}, TierHigh},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTier(tc.profile)
			if got != tc.want {
				t.Errorf("classifyTier(%+v) = %v, want %v", tc.profile, got, tc.want)
			}
		})
	}
}

func TestHardwareTierString(t *testing.T) {
	tests := []struct {
		tier HardwareTier
		want string
	}{
		{TierConstrained, "constrained"},
		{TierStandard, "standard"},
		{TierCapable, "capable"},
		{TierHigh, "high"},
		{HardwareTier(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("HardwareTier(%d).String() = %q, want %q", tc.tier, got, tc.want)
		}
	}
}
