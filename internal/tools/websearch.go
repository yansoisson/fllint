package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const ollamaSearchURL = "https://ollama.com/api/web_search"
const ollamaFetchURL = "https://ollama.com/api/web_fetch"

// SearchResult represents a single web search result from Ollama's API.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// FetchResult represents a fetched web page from Ollama's API.
type FetchResult struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Links   []string `json:"links"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// WebSearch calls Ollama's web search API.
func WebSearch(ctx context.Context, apiKey, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"query":       query,
		"max_results": maxResults,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaSearchURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("could not create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("Web search authentication failed. Check your Ollama API key.")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("web search error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not parse search response: %w", err)
	}
	return result.Results, nil
}

// WebFetch calls Ollama's web fetch API to retrieve a single page.
func WebFetch(ctx context.Context, apiKey, url string) (*FetchResult, error) {
	payload, _ := json.Marshal(map[string]string{"url": url})

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaFetchURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("could not create fetch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web fetch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("Web fetch authentication failed. Check your Ollama API key.")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("web fetch error %d: %s", resp.StatusCode, string(body))
	}

	var result FetchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("could not parse fetch response: %w", err)
	}
	return &result, nil
}

// ToolDefinitions returns OpenAI-format tool definitions for web_search and web_fetch.
func ToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "web_search",
				"description": "Search the web for current information on a topic. Use this when the user asks about recent events, current facts, or anything that requires up-to-date information.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query",
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of results to return (1-10, default 5)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "web_fetch",
				"description": "Fetch and read the content of a specific web page by URL. Use this when you need to read the full content of a page found via web_search or provided by the user.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL of the web page to fetch",
						},
					},
					"required": []string{"url"},
				},
			},
		},
	}
}

// ExecuteToolCall executes a tool call by name with the given arguments.
// Returns the result as a string suitable for sending back to the model.
func ExecuteToolCall(ctx context.Context, apiKey, toolName, argsJSON string) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	switch toolName {
	case "web_search":
		query, _ := args["query"].(string)
		if query == "" {
			return "", fmt.Errorf("web_search requires a 'query' argument")
		}
		maxResults := 5
		if mr, ok := args["max_results"].(float64); ok {
			maxResults = int(mr)
		}
		results, err := WebSearch(ctx, apiKey, query, maxResults)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(results)
		return string(out), nil

	case "web_fetch":
		url, _ := args["url"].(string)
		if url == "" {
			return "", fmt.Errorf("web_fetch requires a 'url' argument")
		}
		result, err := WebFetch(ctx, apiKey, url)
		if err != nil {
			return "", err
		}
		// Truncate content to avoid overwhelming the context
		content := result.Content
		if len(content) > 8000 {
			content = content[:8000] + "\n\n[Content truncated]"
		}
		out, _ := json.Marshal(map[string]interface{}{
			"title":   result.Title,
			"content": content,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}
