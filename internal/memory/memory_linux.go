//go:build linux

package memory

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Query returns the current system memory info on Linux.
// Attempts to detect NVIDIA GPU VRAM via nvidia-smi.
func Query() (*MemoryInfo, error) {
	info := &MemoryInfo{}

	// Parse /proc/meminfo for RAM
	total, available, err := parseMeminfo()
	if err != nil {
		return nil, fmt.Errorf("parse /proc/meminfo: %w", err)
	}
	info.TotalRAM = total
	info.AvailableRAM = available

	// Try to detect NVIDIA VRAM
	totalVRAM, availableVRAM, err := getNvidiaVRAM()
	if err == nil {
		info.TotalVRAM = totalVRAM
		info.AvailableVRAM = availableVRAM
	}
	// If nvidia-smi fails, VRAM stays 0 (no discrete GPU or no nvidia driver)

	return info, nil
}

func parseMeminfo() (total int64, available int64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if val, ok := parseMeminfoLine(line, "MemTotal:"); ok {
			total = val * 1024 // /proc/meminfo reports in kB
		} else if val, ok := parseMeminfoLine(line, "MemAvailable:"); ok {
			available = val * 1024
		}
	}
	return total, available, scanner.Err()
}

func parseMeminfoLine(line, prefix string) (int64, bool) {
	if !strings.HasPrefix(line, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSpace(rest)
	rest = strings.TrimSuffix(rest, " kB")
	val, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// getNvidiaVRAM queries nvidia-smi for GPU memory.
func getNvidiaVRAM() (total int64, available int64, err error) {
	// nvidia-smi --query-gpu=memory.total,memory.free --format=csv,noheader,nounits
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=memory.total,memory.free",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return 0, 0, err
	}

	// Output: "24576, 24000" (in MiB)
	line := strings.TrimSpace(string(out))
	// If multiple GPUs, just use the first line
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}

	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected nvidia-smi output: %q", line)
	}

	totalMiB, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, err
	}
	freeMiB, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	return totalMiB * 1024 * 1024, freeMiB * 1024 * 1024, nil
}
