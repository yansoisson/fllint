package provider

import "context"

// ProviderClient abstracts provider-specific operations (test connection, list models).
type ProviderClient interface {
	TestConnection(ctx context.Context) error
	ListModels(ctx context.Context) ([]ProviderModel, error)
}

// ProviderModel is a unified model description returned by all provider clients.
type ProviderModel struct {
	Name string `json:"name"`
	Size int64  `json:"size,omitempty"`
}

// NewClient creates the appropriate client for a provider based on its type.
func NewClient(p Provider) ProviderClient {
	switch p.Type {
	case ProviderOpenAI, ProviderOpenRouter:
		return NewOpenAIClient(p.BaseURL, p.APIKey)
	case ProviderAnthropic:
		return NewAnthropicClient(p.BaseURL, p.APIKey)
	default:
		// Ollama (local and cloud) and any unknown types
		return NewOllamaClientAdapter(p.BaseURL, p.APIKey)
	}
}
