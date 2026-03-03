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
}

// Manager handles model discovery and engine lifecycle.
type Manager struct {
	mu sync.RWMutex

	engine   Engine
	models   []ModelInfo
	activeID string

	serverBinaryPath string
	modelsDir        string
	hasBinary        bool
}

// NewManager creates a Manager that discovers models on disk and checks for
// the llama-server binary.
func NewManager(serverBinaryPath string, modelsDir string) *Manager {
	m := &Manager{
		serverBinaryPath: serverBinaryPath,
		modelsDir:        modelsDir,
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

// scanModels finds all .gguf files in the models directory.
func (m *Manager) scanModels() []ModelInfo {
	entries, err := os.ReadDir(m.modelsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Could not read models directory %q: %v", m.modelsDir, err)
		}
		return nil
	}

	var models []ModelInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		models = append(models, ModelInfo{
			ID:       entry.Name(),
			Name:     modelNameFromFilename(entry.Name()),
			Tier:     tierFromSize(info.Size()),
			FilePath: filepath.Join(m.modelsDir, entry.Name()),
			Size:     info.Size(),
			Active:   false,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Size < models[j].Size
	})

	return models
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
		CtxSize:          4096,
		NGPULayers:       999,
		FlashAttn:        true,
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
