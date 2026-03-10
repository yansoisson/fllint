//go:build darwin

package memory

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Query returns the current system memory info on macOS.
// On Apple Silicon, all memory is unified (shared between CPU and GPU).
func Query() (*MemoryInfo, error) {
	info := &MemoryInfo{}

	// Total RAM via sysctl
	total, err := getTotalRAM()
	if err != nil {
		return nil, fmt.Errorf("get total RAM: %w", err)
	}
	info.TotalRAM = total

	// Available RAM via vm_stat
	available, err := getAvailableRAM()
	if err != nil {
		return nil, fmt.Errorf("get available RAM: %w", err)
	}
	info.AvailableRAM = available

	// Apple Silicon = unified memory (VRAM = RAM)
	if runtime.GOARCH == "arm64" {
		info.IsUnified = true
		info.TotalVRAM = info.TotalRAM
		info.AvailableVRAM = info.AvailableRAM
	}

	return info, nil
}

func getTotalRAM() (int64, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// getAvailableRAM parses vm_stat output to compute available memory.
func getAvailableRAM() (int64, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(out), "\n")

	// Parse page size from first line: "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
	var pageSize int64 = 16384
	if len(lines) > 0 {
		if idx := strings.Index(lines[0], "page size of "); idx >= 0 {
			rest := lines[0][idx+len("page size of "):]
			if end := strings.Index(rest, " "); end >= 0 {
				if ps, err := strconv.ParseInt(rest[:end], 10, 64); err == nil {
					pageSize = ps
				}
			}
		}
	}

	var free, inactive, speculative int64
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if val, ok := parseVMStatLine(line, "Pages free:"); ok {
			free = val
		} else if val, ok := parseVMStatLine(line, "Pages inactive:"); ok {
			inactive = val
		} else if val, ok := parseVMStatLine(line, "Pages speculative:"); ok {
			speculative = val
		}
	}

	available := (free + inactive + speculative) * pageSize
	return available, nil
}

func parseVMStatLine(line, prefix string) (int64, bool) {
	if !strings.HasPrefix(line, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSpace(rest)
	rest = strings.TrimSuffix(rest, ".")
	val, err := strconv.ParseInt(rest, 10, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}
