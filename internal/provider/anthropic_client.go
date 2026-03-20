package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicVersion = "2023-06-01"

// AnthropicClient talks to the Anthropic API for model listing and connection testing.
type AnthropicClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewAnthropicClient creates a client for the Anthropic API.
func NewAnthropicClient(baseURL, apiKey string) ProviderClient {
	return &AnthropicClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *AnthropicClient) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/models", nil)
	if err != nil {
		return fmt.Errorf("Could not create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Could not connect to %s. Check the URL and your internet connection.", c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("Authentication failed. Check your API key.")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Server returned unexpected status %d.", resp.StatusCode)
	}
	return nil
}

func (c *AnthropicClient) ListModels(ctx context.Context) ([]ProviderModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("Could not create request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Could not connect to %s. Check the URL and your internet connection.", c.baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("Authentication failed. Check your API key.")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Server returned error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Could not parse response: %w", err)
	}

	models := make([]ProviderModel, len(result.Data))
	for i, m := range result.Data {
		models[i] = ProviderModel{Name: m.ID}
	}
	return models, nil
}

func (c *AnthropicClient) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
	}
}
