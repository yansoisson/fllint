package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient talks to OpenAI-compatible APIs (OpenAI, OpenRouter).
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewOpenAIClient creates a client for an OpenAI-compatible API.
func NewOpenAIClient(baseURL, apiKey string) ProviderClient {
	return &OpenAIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *OpenAIClient) TestConnection(ctx context.Context) error {
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

func (c *OpenAIClient) ListModels(ctx context.Context) ([]ProviderModel, error) {
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

func (c *OpenAIClient) setAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
