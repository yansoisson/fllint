package llm

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fllint/fllint/internal/config"
	"github.com/fllint/fllint/internal/memory"
)

// MemoryError is returned when there is not enough memory to load a model.
// The frontend uses this to show the Pro Mode unload popup.
type MemoryError struct {
	ModelName      string `json:"model_name"`
	RequiredBytes  int64  `json:"required_bytes"`
	AvailableBytes int64  `json:"available_bytes"`
}

func (e *MemoryError) Error() string {
	return fmt.Sprintf(
		"Not enough memory to load %s. Needs ~%.1f GB, %.1f GB available.",
		e.ModelName,
		float64(e.RequiredBytes)/(1024*1024*1024),
		float64(e.AvailableBytes)/(1024*1024*1024),
	)
}

// EngineEntry represents a running llama-server process for a specific model.
type EngineEntry struct {
	Engine     *LlamaCppEngine
	ModelID    string
	Loading    bool      // true while the engine is starting up
	LastUsedAt time.Time // updated when engine is retrieved for inference
}

// EngineStatusInfo describes the status of a single loaded engine.
type EngineStatusInfo struct {
	ModelID      string  `json:"model_id"`
	ModelName    string  `json:"model_name"`
	State        string  `json:"engine_state"`
	Error        string  `json:"error,omitempty"`
	HasVision    bool    `json:"has_vision"`
	LoadProgress float64 `json:"load_progress,omitempty"`
}

// ManagerStatus represents the overall system status visible to the frontend.
type ManagerStatus struct {
	Engines        []EngineStatusInfo `json:"engines"`
	DefaultModelID string             `json:"default_model_id,omitempty"`
	HasBinary      bool               `json:"has_binary"`
	HasModels      bool               `json:"has_models"`

	// Backward-compat fields: reflect the default engine's status
	EngineState  string  `json:"engine_state"`
	Error        string  `json:"error,omitempty"`
	ModelName    string  `json:"model_name,omitempty"`
	HasVision    bool    `json:"has_vision"`
	LoadProgress float64 `json:"load_progress,omitempty"`
}

// Manager handles model discovery and engine lifecycle.
// It supports multiple concurrent llama-server processes, one per loaded model.
type Manager struct {
	mu       sync.RWMutex
	loadMu   sync.Mutex // serialises LoadModel/UnloadModel calls without blocking reads

	engines        map[string]*EngineEntry // modelID -> running engine
	models         []ModelInfo
	defaultModelID string // the "active" model for backward compat

	serverBinaryPath string
	modelsDir        string
	dataDir          string
	hasBinary        bool
}

// NewManager creates a Manager that discovers models on disk and checks for
// the llama-server binary.
func NewManager(serverBinaryPath string, modelsDir string, dataDir string) *Manager {
	m := &Manager{
		serverBinaryPath: serverBinaryPath,
		modelsDir:        modelsDir,
		dataDir:          dataDir,
		engines:          make(map[string]*EngineEntry),
	}

	if _, err := os.Stat(serverBinaryPath); err == nil {
		m.hasBinary = true
	} else {
		log.Printf("llama-server not found at %q — models will not load until it is placed there", serverBinaryPath)
	}

	m.models = m.scanModels()

	if len(m.models) == 0 {
		log.Printf("No .gguf model files found in %q", modelsDir)
	} else {
		log.Printf("Found %d model(s) in %s", len(m.models), modelsDir)
	}

	return m
}

// scanModels finds all model .gguf files in the models directory.
// It supports two layouts:
//
//	models/                         (flat — loose files)
//	  model.gguf
//	  mmproj.gguf
//
//	models/                         (per-model subdirectories)
//	  Qwen3.5-2B/
//	    Qwen3.5-2B-Q8_0.gguf
//	    mmproj-BF16.gguf
//
// Inside a subdirectory the mmproj is paired with the model automatically.
// Loose files at the top level use filename-based matching as a fallback.
func (m *Manager) scanModels() []ModelInfo {
	entries, err := os.ReadDir(m.modelsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Could not read models directory %q: %v", m.modelsDir, err)
		}
		return nil
	}

	var models []ModelInfo

	// Loose top-level files (backwards compat)
	var looseModels []ModelInfo
	var looseMmproj []string

	for _, entry := range entries {
		if entry.IsDir() {
			// Scan subdirectory for a model + optional mmproj
			found := m.scanModelDir(filepath.Join(m.modelsDir, entry.Name()))
			if found != nil {
				models = append(models, *found)
			}
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			continue
		}

		lower := strings.ToLower(entry.Name())
		if strings.Contains(lower, "mmproj") {
			looseMmproj = append(looseMmproj, filepath.Join(m.modelsDir, entry.Name()))
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		looseModels = append(looseModels, ModelInfo{
			ID:       entry.Name(),
			Name:     modelNameFromFilename(entry.Name()),
			Tier:     tierFromSize(info.Size()),
			FilePath: filepath.Join(m.modelsDir, entry.Name()),
			Size:     info.Size(),
		})
	}

	// Match loose mmproj files to loose models
	matchMmprojToModels(looseModels, looseMmproj)
	models = append(models, looseModels...)

	sort.Slice(models, func(i, j int) bool {
		return models[i].Size < models[j].Size
	})

	return models
}

// scanModelDir scans a single subdirectory for one model .gguf and an
// optional mmproj .gguf. Returns nil if no model file is found.
func (m *Manager) scanModelDir(dir string) *ModelInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var modelFile os.DirEntry
	var mmprojPath string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			continue
		}
		if strings.Contains(strings.ToLower(entry.Name()), "mmproj") {
			mmprojPath = filepath.Join(dir, entry.Name())
		} else if modelFile == nil {
			modelFile = entry
		}
	}

	if modelFile == nil {
		return nil
	}

	info, err := modelFile.Info()
	if err != nil {
		return nil
	}

	// Load or create model.json for display name
	meta := loadOrCreateModelMeta(dir, modelNameFromFilename(modelFile.Name()))

	// Use "dirname/filename" as the ID so it stays unique across folders
	dirName := filepath.Base(dir)
	mi := &ModelInfo{
		ID:       dirName + "/" + modelFile.Name(),
		Name:     meta.Name,
		FilePath: filepath.Join(dir, modelFile.Name()),
		Size:     info.Size(),
	}

	// Classify tier by directory name: Lite/Standard/Pro get their tier,
	// everything else is Custom
	if tier := tierFromDirName(dirName); tier != "" {
		mi.Tier = tier
	} else {
		mi.Tier = TierCustom
	}

	if mmprojPath != "" {
		mi.MmprojPath = mmprojPath
		mi.Vision = true
		log.Printf("Paired mmproj %s with model %s", filepath.Base(mmprojPath), mi.Name)
	}

	return mi
}

// matchMmprojToModels pairs loose mmproj files with loose model files.
func matchMmprojToModels(models []ModelInfo, mmprojPaths []string) {
	if len(mmprojPaths) == 0 || len(models) == 0 {
		return
	}

	// Simple case: one mmproj + one model → pair them
	if len(mmprojPaths) == 1 && len(models) == 1 {
		models[0].MmprojPath = mmprojPaths[0]
		models[0].Vision = true
		log.Printf("Paired mmproj %s with model %s", filepath.Base(mmprojPaths[0]), models[0].Name)
		return
	}

	// Otherwise match by filename prefix overlap
	for i := range models {
		modelBase := strings.ToLower(strings.TrimSuffix(models[i].ID, filepath.Ext(models[i].ID)))
		for _, mp := range mmprojPaths {
			mpBase := strings.ToLower(filepath.Base(mp))
			if strings.Contains(mpBase, modelBase) || commonPrefixLen(modelBase, mpBase) >= 8 {
				models[i].MmprojPath = mp
				models[i].Vision = true
				log.Printf("Paired mmproj %s with model %s", filepath.Base(mp), models[i].Name)
				break
			}
		}
	}
}

// commonPrefixLen returns the number of leading characters two strings share.
func commonPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// RefreshModels rescans the models directory. Useful after the user adds
// new model files while the app is running.
func (m *Manager) RefreshModels() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Also re-check the binary in case user just placed it
	if _, err := os.Stat(m.serverBinaryPath); err == nil {
		m.hasBinary = true
	}

	m.models = m.scanModels()

	// Mark models that have running engines or are the default as active
	for i := range m.models {
		if m.models[i].ID == m.defaultModelID {
			m.models[i].Active = true
		}
	}
}

// findModel returns a pointer to the ModelInfo with the given ID, or nil.
// Must be called with m.mu held (read or write).
func (m *Manager) findModel(modelID string) *ModelInfo {
	for i := range m.models {
		if m.models[i].ID == modelID {
			return &m.models[i]
		}
	}
	return nil
}

// LoadModel starts a new llama-server process for the given model if it is
// not already running. If the model is already loaded (or loading), this
// returns nil immediately.
//
// The slow engine Start() call runs outside the RWMutex so that read-only
// endpoints (ListModels, Status, etc.) remain responsive.
func (m *Manager) LoadModel(modelID string) error {
	// loadMu serialises concurrent LoadModel/UnloadModel calls without
	// blocking readers on m.mu.
	m.loadMu.Lock()
	defer m.loadMu.Unlock()

	// --- Phase 1: validate & check if already loaded (short read lock) ---
	m.mu.RLock()
	if entry, ok := m.engines[modelID]; ok {
		m.mu.RUnlock()
		if entry.Loading {
			log.Printf("Model %q is already loading", modelID)
		} else {
			log.Printf("Model %q is already loaded", modelID)
		}
		return nil
	}
	target := m.findModel(modelID)
	if target == nil {
		m.mu.RUnlock()
		return fmt.Errorf("Model %q not found. Try refreshing the model list.", modelID)
	}
	if !m.hasBinary {
		m.mu.RUnlock()
		return fmt.Errorf(
			"Cannot load model: llama-server binary not found at %q. "+
				"Download it from https://github.com/ggml-org/llama.cpp/releases "+
				"and place it in the bin/ folder",
			m.serverBinaryPath,
		)
	}

	// Snapshot target fields so we can use them after releasing the lock.
	targetFilePath := target.FilePath
	targetName := target.Name
	targetSize := target.Size
	targetMmprojPath := target.MmprojPath

	// Compute memory used by already-loaded engines
	var usedMemory int64
	for _, entry := range m.engines {
		if mi := m.findModel(entry.ModelID); mi != nil {
			usedMemory += memory.EstimateModelRAM(mi.Size)
		}
	}
	m.mu.RUnlock()

	// --- Memory check ---
	// Use TotalRAM - 5GB as the budget (macOS AvailableRAM is misleadingly low).
	cfg := config.Get()
	requiredMemory := memory.EstimateModelRAM(targetSize)
	memInfo, memErr := memory.Query()

	if memErr == nil {
		budget := memory.ModelBudget(memInfo)
		if requiredMemory+usedMemory > budget && !cfg.ProMode {
			// Try auto-unloading LRU models to free space.
			// Release loadMu temporarily since autoUnloadForSpace calls UnloadModel.
			m.loadMu.Unlock()
			m.autoUnloadForSpace(requiredMemory)
			m.loadMu.Lock()

			// Recalculate used memory after unloading
			m.mu.RLock()
			usedMemory = 0
			for _, entry := range m.engines {
				if mi := m.findModel(entry.ModelID); mi != nil {
					usedMemory += memory.EstimateModelRAM(mi.Size)
				}
			}
			m.mu.RUnlock()
		}
		if requiredMemory+usedMemory > budget {
			return &MemoryError{
				ModelName:      targetName,
				RequiredBytes:  requiredMemory,
				AvailableBytes: budget - usedMemory,
			}
		}
	} else {
		log.Printf("WARNING: Could not query system memory: %v", memErr)
	}

	// --- Phase 2: register a loading placeholder (short write lock) ---
	m.mu.Lock()
	// Double-check: another goroutine may have loaded this model while we
	// were waiting on loadMu.
	if _, ok := m.engines[modelID]; ok {
		m.mu.Unlock()
		return nil
	}
	m.engines[modelID] = &EngineEntry{
		ModelID: modelID,
		Loading: true,
	}
	m.mu.Unlock()

	// Helper to remove the placeholder on failure.
	removeOnFail := func() {
		m.mu.Lock()
		delete(m.engines, modelID)
		m.mu.Unlock()
	}

	// --- Phase 3: create & start new engine (no lock needed) ---
	log.Printf("Loading model %q (%s)...", targetName, modelID)
	engine, err := NewLlamaCppEngine(LlamaCppConfig{
		ServerBinaryPath: m.serverBinaryPath,
		ModelPath:        targetFilePath,
		ModelName:        targetName,
		MmprojPath:       targetMmprojPath,
		CtxSize:          cfg.CtxSize,
		NGPULayers:       cfg.NGPULayers,
		FlashAttn:        cfg.FlashAttn,
		DataDir:          m.dataDir,
		InferenceParams: InferenceParams{
			Temperature:   cfg.Temperature,
			TopP:          cfg.TopP,
			TopK:          cfg.TopK,
			RepeatPenalty: cfg.RepeatPenalty,
			MaxTokens:     cfg.MaxTokens,
			Seed:          cfg.Seed,
		},
	})
	if err != nil {
		removeOnFail()
		return err
	}
	if err := engine.Start(); err != nil {
		removeOnFail()
		return err
	}

	// --- Phase 4: install the engine (short write lock) ---
	m.mu.Lock()
	m.engines[modelID] = &EngineEntry{
		Engine:     engine,
		ModelID:    modelID,
		Loading:    false,
		LastUsedAt: time.Now(),
	}
	m.mu.Unlock()
	log.Printf("Model %q loaded (process launched)", targetName)

	return nil
}

// UnloadModel stops the llama-server process for the given model.
// Returns an error if the model is not currently loaded.
func (m *Manager) UnloadModel(modelID string) error {
	m.loadMu.Lock()
	defer m.loadMu.Unlock()

	// --- Phase 1: detach the engine (short write lock) ---
	m.mu.Lock()
	entry, ok := m.engines[modelID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("Model %q is not currently loaded.", modelID)
	}
	delete(m.engines, modelID)

	// If this was the default model, clear the default
	if m.defaultModelID == modelID {
		m.defaultModelID = ""
		for i := range m.models {
			if m.models[i].ID == modelID {
				m.models[i].Active = false
			}
		}
	}
	m.mu.Unlock()

	// --- Phase 2: stop the engine without holding the lock ---
	if entry.Engine != nil {
		log.Printf("Unloading model %q...", modelID)
		entry.Engine.Stop()
		log.Printf("Model %q unloaded", modelID)
	}

	return nil
}

// autoUnloadForSpace frees memory by unloading least-recently-used engines
// until there is enough room for requiredBytes. Skips the default model.
// Must be called WITHOUT holding loadMu (it calls UnloadModel which takes it).
func (m *Manager) autoUnloadForSpace(requiredBytes int64) {
	m.mu.RLock()
	// Build list of candidates sorted by LastUsedAt ascending (oldest first)
	type candidate struct {
		modelID    string
		lastUsed   time.Time
		memEstimate int64
	}
	var candidates []candidate
	for id, entry := range m.engines {
		if id == m.defaultModelID {
			continue // don't auto-unload the default model
		}
		if entry.Loading {
			continue // don't unload models that are still loading
		}
		var est int64
		if mi := m.findModel(id); mi != nil {
			est = memory.EstimateModelRAM(mi.Size)
		}
		candidates = append(candidates, candidate{
			modelID:    id,
			lastUsed:   entry.LastUsedAt,
			memEstimate: est,
		})
	}
	m.mu.RUnlock()

	// Sort by last used (oldest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].lastUsed.Before(candidates[j].lastUsed)
	})

	// Unload oldest engines until we have enough estimated free memory
	var freed int64
	for _, c := range candidates {
		if freed >= requiredBytes {
			break
		}
		log.Printf("Auto-unloading model %q (LRU) to free ~%.1f GB",
			c.modelID, float64(c.memEstimate)/(1024*1024*1024))
		if err := m.UnloadModel(c.modelID); err != nil {
			log.Printf("WARNING: failed to auto-unload %q: %v", c.modelID, err)
			continue
		}
		freed += c.memEstimate
	}
}

// GetEngine returns the engine for a given model, or nil if not loaded.
// Updates LastUsedAt for LRU tracking.
func (m *Manager) GetEngine(modelID string) Engine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.engines[modelID]; ok && entry.Engine != nil {
		entry.LastUsedAt = time.Now()
		return entry.Engine
	}
	return nil
}

// GetDefaultEngine returns the engine for the default model, or nil.
func (m *Manager) GetDefaultEngine() Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultModelID == "" {
		return nil
	}
	if entry, ok := m.engines[m.defaultModelID]; ok && entry.Engine != nil {
		return entry.Engine
	}
	return nil
}

// Engine returns the current default Engine, or nil if no model is loaded.
// This is the backward-compatible method used by the chat handler.
func (m *Manager) Engine() Engine {
	return m.GetDefaultEngine()
}

// ListModels returns a copy of the discovered model list with loaded status.
func (m *Manager) ListModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ModelInfo, len(m.models))
	copy(result, m.models)
	for i := range result {
		if _, ok := m.engines[result[i].ID]; ok {
			result[i].Loaded = true
		}
	}
	return result
}

// SetActive ensures the given model is loaded, then sets it as the default.
// It does NOT stop other running engines (they may be used by other tabs).
func (m *Manager) SetActive(modelID string) error {
	// LoadModel handles its own locking and is idempotent.
	if err := m.LoadModel(modelID); err != nil {
		return err
	}

	// Set the default model ID and mark it active in the model list.
	m.mu.Lock()
	m.defaultModelID = modelID
	for i := range m.models {
		m.models[i].Active = (m.models[i].ID == modelID)
	}
	m.mu.Unlock()

	log.Printf("Default model set to %q", modelID)
	return nil
}

// IsSwitching reports whether any model is currently loading.
func (m *Manager) IsSwitching() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.engines {
		if entry.Loading {
			return true
		}
	}
	return false
}

// Status returns the current system status for the frontend.
func (m *Manager) Status() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ManagerStatus{
		DefaultModelID: m.defaultModelID,
		HasBinary:      m.hasBinary,
		HasModels:      len(m.models) > 0,
	}

	// Build per-engine status list
	for _, entry := range m.engines {
		info := EngineStatusInfo{
			ModelID: entry.ModelID,
		}
		if entry.Loading {
			info.State = EngineStateStarting.String()
			if mi := m.findModel(entry.ModelID); mi != nil {
				info.ModelName = mi.Name
			}
			if entry.Engine != nil {
				info.LoadProgress = entry.Engine.LoadProgress()
			}
		} else if entry.Engine != nil {
			state, stateErr := entry.Engine.State()
			info.State = state.String()
			info.ModelName = entry.Engine.ModelName()
			info.HasVision = entry.Engine.HasVision()
			if stateErr != nil {
				info.Error = stateErr.Error()
			}
			if state == EngineStateStarting {
				info.LoadProgress = entry.Engine.LoadProgress()
			}
		}
		status.Engines = append(status.Engines, info)
	}

	// Ensure Engines is never null in JSON
	if status.Engines == nil {
		status.Engines = []EngineStatusInfo{}
	}

	// Populate backward-compat fields from the default engine
	if m.defaultModelID != "" {
		if entry, ok := m.engines[m.defaultModelID]; ok {
			if entry.Loading {
				status.EngineState = EngineStateStarting.String()
				if mi := m.findModel(m.defaultModelID); mi != nil {
					status.ModelName = mi.Name
				}
				if entry.Engine != nil {
					status.LoadProgress = entry.Engine.LoadProgress()
				}
			} else if entry.Engine != nil {
				state, stateErr := entry.Engine.State()
				status.EngineState = state.String()
				status.ModelName = entry.Engine.ModelName()
				status.HasVision = entry.Engine.HasVision()
				if stateErr != nil {
					status.Error = stateErr.Error()
				}
				if state == EngineStateStarting {
					status.LoadProgress = entry.Engine.LoadProgress()
				}
			}
		} else {
			// Default model ID is set but not in the engines map (shouldn't happen normally)
			status.EngineState = EngineStateIdle.String()
		}
	} else {
		// No default model — check if any engine is loading for backward compat
		anyLoading := false
		for _, entry := range m.engines {
			if entry.Loading {
				anyLoading = true
				break
			}
		}
		if anyLoading {
			status.EngineState = EngineStateStarting.String()
		} else {
			status.EngineState = EngineStateIdle.String()
		}
	}

	return status
}

// DeleteModel removes a model's files from disk. The model must not be
// currently loaded.
func (m *Manager) DeleteModel(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var target *ModelInfo
	var targetIdx int
	for i := range m.models {
		if m.models[i].ID == modelID {
			target = &m.models[i]
			targetIdx = i
			break
		}
	}
	if target == nil {
		return fmt.Errorf("Model %q not found.", modelID)
	}

	// Cannot delete a model that is currently loaded (running engine)
	if _, loaded := m.engines[modelID]; loaded {
		return fmt.Errorf("Cannot delete a loaded model. Unload it first.")
	}
	if target.Active {
		return fmt.Errorf("Cannot delete the active model. Switch to a different model first.")
	}

	// Delete model file
	if err := os.Remove(target.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Failed to delete model file: %w", err)
	}

	// Delete mmproj if present
	if target.MmprojPath != "" {
		os.Remove(target.MmprojPath)
	}

	// Try removing the subdirectory (only succeeds if empty)
	modelDir := filepath.Dir(target.FilePath)
	if modelDir != m.modelsDir {
		os.Remove(modelDir)
	}

	// Remove from list
	m.models = append(m.models[:targetIdx], m.models[targetIdx+1:]...)
	return nil
}

// RenameModel updates the display name of a model in its model.json file.
func (m *Manager) RenameModel(modelID string, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("Model name cannot be empty.")
	}

	for i := range m.models {
		if m.models[i].ID == modelID {
			modelDir := filepath.Dir(m.models[i].FilePath)
			if modelDir == m.modelsDir {
				return fmt.Errorf("Loose model files cannot be renamed. Move the model to a subfolder first.")
			}

			meta := ModelMeta{Name: newName}
			metaPath := filepath.Join(modelDir, modelMetaFile)
			data, err := json.MarshalIndent(meta, "", "  ")
			if err != nil {
				return fmt.Errorf("Failed to save model name: %w", err)
			}
			if err := os.WriteFile(metaPath, append(data, '\n'), 0644); err != nil {
				return fmt.Errorf("Failed to save model name: %w", err)
			}

			m.models[i].Name = newName
			return nil
		}
	}
	return fmt.Errorf("Model %q not found.", modelID)
}

// Stop gracefully shuts down ALL running engines. Called on app exit.
func (m *Manager) Stop() {
	m.mu.Lock()
	// Collect all engines to stop
	toStop := make([]*LlamaCppEngine, 0, len(m.engines))
	for _, entry := range m.engines {
		if entry.Engine != nil {
			toStop = append(toStop, entry.Engine)
		}
	}
	m.engines = make(map[string]*EngineEntry)
	m.defaultModelID = ""
	m.mu.Unlock()

	// Stop engines outside the lock so reads are not blocked during shutdown
	for _, engine := range toStop {
		engine.Stop()
	}
}

// LoadedModelIDs returns the IDs of all currently loaded (or loading) models.
func (m *Manager) LoadedModelIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.engines))
	for id := range m.engines {
		ids = append(ids, id)
	}
	return ids
}

// MemoryStatus represents memory usage information for the API.
type MemoryStatus struct {
	System      *memory.MemoryInfo  `json:"system"`
	UsedByModels int64              `json:"used_by_models"` // estimated bytes used by loaded models
	Models       []ModelMemoryInfo  `json:"models"`         // per-model estimates
}

// ModelMemoryInfo describes memory usage estimate for a single model.
type ModelMemoryInfo struct {
	ModelID       string `json:"model_id"`
	ModelName     string `json:"model_name"`
	EstimatedRAM  int64  `json:"estimated_ram"`
	Loaded        bool   `json:"loaded"`
}

// MemoryStatus returns current memory info with per-model estimates.
func (m *Manager) MemoryInfo() MemoryStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalUsed int64
	result := MemoryStatus{}

	for _, model := range m.models {
		est := memory.EstimateModelRAM(model.Size)
		_, loaded := m.engines[model.ID]
		if loaded {
			totalUsed += est
		}
		result.Models = append(result.Models, ModelMemoryInfo{
			ModelID:      model.ID,
			ModelName:    model.Name,
			EstimatedRAM: est,
			Loaded:       loaded,
		})
	}

	result.UsedByModels = totalUsed
	if memInfo, err := memory.Query(); err == nil {
		result.System = memInfo
	}

	if result.Models == nil {
		result.Models = []ModelMemoryInfo{}
	}

	return result
}
