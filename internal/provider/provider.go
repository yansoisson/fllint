package provider

// ProviderType identifies the kind of external model server.
type ProviderType string

const (
	ProviderOllamaLocal ProviderType = "ollama-local"
	ProviderOllamaCloud ProviderType = "ollama-cloud"
)

// ProviderTypeInfo describes a provider type's capabilities and UI hints.
type ProviderTypeInfo struct {
	Type        ProviderType `json:"type"`
	Label       string       `json:"label"`
	RequiresKey bool         `json:"requires_key"`
	DefaultURL  string       `json:"default_url"`
}

// RegisteredTypes returns all supported provider types with their metadata.
func RegisteredTypes() []ProviderTypeInfo {
	return []ProviderTypeInfo{
		{
			Type:        ProviderOllamaLocal,
			Label:       "Ollama Local",
			RequiresKey: false,
			DefaultURL:  "http://localhost:11434",
		},
		{
			Type:        ProviderOllamaCloud,
			Label:       "Ollama Cloud",
			RequiresKey: true,
			DefaultURL:  "https://ollama.com",
		},
	}
}

// SelectedModel represents a model the user has chosen to use from a provider.
type SelectedModel struct {
	Name        string `json:"name"`                   // e.g. "llama3.2"
	DisplayName string `json:"display_name,omitempty"` // optional custom name
}

// Provider represents an external model server configuration.
type Provider struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Type    ProviderType    `json:"type"`
	BaseURL string          `json:"base_url"`
	APIKey  string          `json:"api_key,omitempty"`
	Enabled bool            `json:"enabled"`
	Models  []SelectedModel `json:"models"`
}

// ProviderResponse is the API response for a provider (API key redacted).
type ProviderResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Type      ProviderType    `json:"type"`
	BaseURL   string          `json:"base_url"`
	HasAPIKey bool            `json:"has_api_key"`
	Enabled   bool            `json:"enabled"`
	Models    []SelectedModel `json:"models"`
}

// Redacted returns a copy safe for API responses (no API key).
func (p *Provider) Redacted() ProviderResponse {
	models := p.Models
	if models == nil {
		models = []SelectedModel{}
	}
	return ProviderResponse{
		ID:        p.ID,
		Name:      p.Name,
		Type:      p.Type,
		BaseURL:   p.BaseURL,
		HasAPIKey: p.APIKey != "",
		Enabled:   p.Enabled,
		Models:    models,
	}
}
