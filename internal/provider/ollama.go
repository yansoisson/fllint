package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClient talks to an Ollama server for non-streaming operations.
type OllamaClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewOllamaClient creates a client for the given Ollama server.
func NewOllamaClient(baseURL, apiKey string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// OllamaModel describes a model available on the Ollama server.
type OllamaModel struct {
	Name    string `json:"name"`
	Model   string `json:"model"`
	Size    int64  `json:"size"`
	Details struct {
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	} `json:"details"`
}

// ListModels calls GET /api/tags and returns available models.
func (c *OllamaClient) ListModels(ctx context.Context) ([]OllamaModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("Could not create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"Could not connect to Ollama at %s. Make sure Ollama is running.",
			c.baseURL,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("Authentication failed. Check your API key.")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []OllamaModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Could not parse Ollama response: %w", err)
	}

	if result.Models == nil {
		result.Models = []OllamaModel{}
	}

	return result.Models, nil
}

// TestConnection calls GET /api/version to verify the server is reachable.
func (c *OllamaClient) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/version", nil)
	if err != nil {
		return fmt.Errorf("Could not create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"Could not connect to Ollama at %s. Make sure Ollama is running.",
			c.baseURL,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Authentication failed. Check your API key.")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned unexpected status %d.", resp.StatusCode)
	}

	return nil
}

func (c *OllamaClient) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
