package router

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// HardwareTier classifies the machine's capability for model routing.
type HardwareTier int

const (
	// TierConstrained: <4GB RAM or <2 cores — prefer smallest models
	TierConstrained HardwareTier = iota
	// TierStandard: 4-16GB RAM, 2-8 cores — default routing
	TierStandard
	// TierCapable: 16-64GB RAM, 8+ cores — can run larger local models
	TierCapable
	// TierHigh: 64GB+ RAM or discrete GPU — full local model support
	TierHigh
)

func (t HardwareTier) String() string {
	switch t {
	case TierConstrained:
		return "constrained"
	case TierStandard:
		return "standard"
	case TierCapable:
		return "capable"
	case TierHigh:
		return "high"
	default:
		return "unknown"
	}
}

// HardwareProfile describes the detected hardware capabilities.
type HardwareProfile struct {
	CPUCores    int          // logical CPU count
	MemoryMB    int          // total system memory in MB
	GPUMemoryMB int          // discrete GPU VRAM in MB (0 = none detected)
	Tier        HardwareTier // computed tier
}

// DetectHardware probes the system and returns a HardwareProfile.
// All detection is best-effort — failures degrade to TierStandard.
func DetectHardware() HardwareProfile {
	p := HardwareProfile{
		CPUCores: runtime.NumCPU(),
	}
	p.MemoryMB = detectMemoryMB()
	p.GPUMemoryMB = detectGPUMemoryMB()
	p.Tier = classifyTier(p)
	return p
}

// classifyTier assigns a tier based on detected hardware.
func classifyTier(p HardwareProfile) HardwareTier {
	if p.GPUMemoryMB >= 8192 || p.MemoryMB >= 65536 {
		return TierHigh
	}
	if p.MemoryMB >= 16384 || p.CPUCores >= 8 {
		return TierCapable
	}
	if p.MemoryMB < 4096 || p.CPUCores < 2 {
		return TierConstrained
	}
	return TierStandard
}

// detectMemoryMB returns total system memory in MB.
func detectMemoryMB() int {
	switch runtime.GOOS {
	case "linux":
		return detectMemoryLinux()
	case "darwin":
		return detectMemoryDarwin()
	default:
		return 0
	}
}

func detectMemoryLinux() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.Atoi(fields[1])
				if err == nil {
					return kb / 1024
				}
			}
		}
	}
	return 0
}

func detectMemoryDarwin() int {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return int(bytes / (1024 * 1024))
}

// detectGPUMemoryMB probes for discrete GPU VRAM via nvidia-smi.
func detectGPUMemoryMB() int {
	out, err := exec.Command("nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return 0
	}
	// Take first GPU if multiple
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	mb, err := strconv.Atoi(line)
	if err != nil {
		return 0
	}
	return mb
}
