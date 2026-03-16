package memory

// MemoryInfo describes the system's available memory.
type MemoryInfo struct {
	TotalRAM      int64 `json:"total_ram"`      // bytes
	AvailableRAM  int64 `json:"available_ram"`  // bytes
	TotalVRAM     int64 `json:"total_vram"`     // 0 on Mac (unified memory)
	AvailableVRAM int64 `json:"available_vram"` // 0 on Mac (unified memory)
	IsUnified     bool  `json:"is_unified"`     // true on macOS Apple Silicon
}

// SystemReserveBytes is the amount of RAM reserved for the OS and other apps.
// macOS aggressively caches, so vm_stat's "available" is misleadingly low.
// Using TotalRAM - reserve gives a much more accurate model budget.
const SystemReserveBytes int64 = 5 * 1024 * 1024 * 1024 // 5 GB

// ModelBudget returns the total RAM available for loading models.
// This is TotalRAM minus a fixed system reserve, which is more reliable
// than the OS-reported "available" memory (especially on macOS).
func ModelBudget(info *MemoryInfo) int64 {
	budget := info.TotalRAM - SystemReserveBytes
	if budget < 0 {
		budget = 0
	}
	return budget
}

// EstimateModelRAM returns a rough estimate of how much RAM a GGUF model
// will need when fully loaded. Uses file size × 1.05 as a heuristic
// (accounts for KV cache, context window overhead, etc.).
func EstimateModelRAM(fileSizeBytes int64) int64 {
	return int64(float64(fileSizeBytes) * 1.05)
}

// EstimateGPULayers calculates how many GPU layers can fit in the available
// memory. totalLayers is the maximum (e.g. 999 for "all"). Returns the
// number of layers that fit, and whether partial offload is needed.
//
// The calculation works by assuming layers are roughly evenly distributed
// across the model's total RAM requirement. We compute the fraction of
// available memory vs required memory and scale the layers accordingly.
func EstimateGPULayers(availableBytes int64, requiredBytes int64, totalLayers int) (layers int, partial bool) {
	if requiredBytes <= 0 || availableBytes >= requiredBytes {
		return totalLayers, false
	}
	if availableBytes <= 0 {
		return 0, true
	}
	// Compute fraction that fits in memory, leave 10% headroom
	usable := float64(availableBytes) * 0.9
	fraction := usable / float64(requiredBytes)
	if fraction >= 1.0 {
		return totalLayers, false
	}
	layers = int(fraction * float64(totalLayers))
	if layers < 0 {
		layers = 0
	}
	return layers, true
}
