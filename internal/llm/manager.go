package llm

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ManagerStatus represents the overall system status visible to the frontend.
type ManagerStatus struct {
	EngineState string `json:"engine_state"`
	Error       string `json:"error,omitempty"`
	ModelName   string `json:"model_name,omitempty"`
	HasBinary   bool   `json:"has_binary"`
	HasModels   bool   `json:"has_models"`
	HasVision   bool   `json:"has_vision"`
}

// Manager handles model discovery and engine lifecycle.
type Manager struct {
	mu sync.RWMutex

	engine   Engine
	models   []ModelInfo
	activeID string

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

	// Use "dirname/filename" as the ID so it stays unique across folders
	dirName := filepath.Base(dir)
	mi := &ModelInfo{
		ID:       dirName + "/" + modelFile.Name(),
		Name:     modelNameFromFilename(modelFile.Name()),
		Tier:     tierFromSize(info.Size()),
		FilePath: filepath.Join(dir, modelFile.Name()),
		Size:     info.Size(),
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

	currentActive := m.activeID
	m.models = m.scanModels()

	for i := range m.models {
		if m.models[i].ID == currentActive {
			m.models[i].Active = true
		}
	}
}

// Engine returns the current active Engine, or nil if no model is loaded.
func (m *Manager) Engine() Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.engine
}

// ListModels returns a copy of the discovered model list.
func (m *Manager) ListModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ModelInfo, len(m.models))
	copy(result, m.models)
	return result
}

// SetActive stops any running engine and starts a new one for the given model.
// This blocks until the server process is launched (but not until it's healthy —
// health polling happens in the background).
func (m *Manager) SetActive(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var target *ModelInfo
	for i := range m.models {
		if m.models[i].ID == modelID {
			target = &m.models[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("Model %q not found. Try refreshing the model list.", modelID)
	}

	if !m.hasBinary {
		return fmt.Errorf(
			"Cannot load model: llama-server binary not found at %q. "+
				"Download it from https://github.com/ggml-org/llama.cpp/releases "+
				"and place it in the bin/ folder",
			m.serverBinaryPath,
		)
	}

	// Stop current engine if running
	if e, ok := m.engine.(*LlamaCppEngine); ok {
		e.Stop()
	}

	engine, err := NewLlamaCppEngine(LlamaCppConfig{
		ServerBinaryPath: m.serverBinaryPath,
		ModelPath:        target.FilePath,
		ModelName:        target.Name,
		MmprojPath:       target.MmprojPath,
		CtxSize:          4096,
		NGPULayers:       999,
		FlashAttn:        true,
		DataDir:          m.dataDir,
	})
	if err != nil {
		m.engine = nil
		return err
	}

	if err := engine.Start(); err != nil {
		m.engine = nil
		return err
	}

	m.engine = engine

	for i := range m.models {
		m.models[i].Active = (m.models[i].ID == modelID)
	}
	m.activeID = modelID

	return nil
}

// Status returns the current system status for the frontend.
func (m *Manager) Status() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ManagerStatus{
		HasBinary: m.hasBinary,
		HasModels: len(m.models) > 0,
	}

	if m.engine == nil {
		status.EngineState = EngineStateIdle.String()
		return status
	}

	if e, ok := m.engine.(*LlamaCppEngine); ok {
		state, stateErr := e.State()
		status.EngineState = state.String()
		status.ModelName = e.ModelName()
		status.HasVision = e.HasVision()
		if stateErr != nil {
			status.Error = stateErr.Error()
		}
	} else {
		// StubEngine or other — always report ready
		status.EngineState = EngineStateReady.String()
		status.ModelName = m.engine.ModelName()
	}

	return status
}

// Stop gracefully shuts down the active engine. Called on app exit.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.engine.(*LlamaCppEngine); ok {
		e.Stop()
	}
	m.engine = nil
}
