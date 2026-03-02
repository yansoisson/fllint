package llm

import (
	"fmt"
	"sync"
)

// Manager handles model discovery and engine switching.
type Manager struct {
	mu       sync.RWMutex
	engine   Engine
	models   []ModelInfo
	activeID string
}

func NewManager() *Manager {
	return &Manager{
		engine: NewStubEngine(),
		models: []ModelInfo{
			{ID: "stub-lite", Name: "Stub Lite", Tier: TierLite, Active: true},
			{ID: "stub-standard", Name: "Stub Standard", Tier: TierStandard, Active: false},
			{ID: "stub-pro", Name: "Stub Pro", Tier: TierPro, Active: false},
		},
		activeID: "stub-lite",
	}
}

func (m *Manager) Engine() Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.engine
}

func (m *Manager) ListModels() []ModelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ModelInfo, len(m.models))
	copy(result, m.models)
	return result
}

func (m *Manager) SetActive(modelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.models {
		if m.models[i].ID == modelID {
			for j := range m.models {
				m.models[j].Active = false
			}
			m.models[i].Active = true
			m.activeID = modelID
			return nil
		}
	}
	return fmt.Errorf("model %q not found", modelID)
}
