package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
)

// parseOpenAISSE reads an OpenAI-format SSE stream and sends tokens to ch.
// Shared between LlamaCppEngine and ExternalEngine.
// The caller should NOT close ch — this function closes it on return.
func parseOpenAISSE(ctx context.Context, body io.ReadCloser, ch chan<- Token) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lastFinishReason string

	// Accumulate tool calls across chunks (they arrive incrementally)
	type toolCallAccum struct {
		ID       string
		Name     string
		ArgsBuilder strings.Builder
	}
	var toolCallAccums []toolCallAccum

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			// Emit accumulated tool calls if any
			if len(toolCallAccums) > 0 {
				var calls []ToolCall
				for _, tc := range toolCallAccums {
					calls = append(calls, ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.ArgsBuilder.String(),
					})
				}
				select {
				case ch <- Token{ToolCalls: calls, FinishReason: lastFinishReason}:
				case <-ctx.Done():
				}
			}
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string             `json:"content"`
					ReasoningContent string             `json:"reasoning_content"` // llama-server format
					Reasoning        string             `json:"reasoning"`         // Ollama format
					ToolCalls        []oaiToolCallChunk  `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Track finish_reason across chunks
		if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
			lastFinishReason = *chunk.Choices[0].FinishReason
		}

		// Usage info from the final chunk (when stream_options.include_usage is true)
		if chunk.Usage != nil {
			// Emit accumulated tool calls along with usage
			var calls []ToolCall
			if len(toolCallAccums) > 0 {
				for _, tc := range toolCallAccums {
					calls = append(calls, ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.ArgsBuilder.String(),
					})
				}
				toolCallAccums = nil
			}
			tok := Token{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				FinishReason:     lastFinishReason,
				ToolCalls:        calls,
			}
			select {
			case <-ctx.Done():
				return
			case ch <- tok:
			}
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		// Accumulate tool call chunks (they arrive incrementally)
		for _, tc := range chunk.Choices[0].Delta.ToolCalls {
			for tc.Index >= len(toolCallAccums) {
				toolCallAccums = append(toolCallAccums, toolCallAccum{})
			}
			if tc.ID != "" {
				toolCallAccums[tc.Index].ID = tc.ID
			}
			if tc.Function.Name != "" {
				toolCallAccums[tc.Index].Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCallAccums[tc.Index].ArgsBuilder.WriteString(tc.Function.Arguments)
			}
		}

		content := chunk.Choices[0].Delta.Content
		// Support both reasoning field names (llama-server vs Ollama)
		reasoning := chunk.Choices[0].Delta.ReasoningContent
		if reasoning == "" {
			reasoning = chunk.Choices[0].Delta.Reasoning
		}
		if content == "" && reasoning == "" {
			continue
		}

		select {
		case <-ctx.Done():
			return
		case ch <- Token{Content: content, Reasoning: reasoning}:
		}
	}

	// If stream ended without [DONE], still emit accumulated tool calls
	if len(toolCallAccums) > 0 {
		var calls []ToolCall
		for _, tc := range toolCallAccums {
			calls = append(calls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.ArgsBuilder.String(),
			})
		}
		select {
		case ch <- Token{ToolCalls: calls, FinishReason: lastFinishReason}:
		case <-ctx.Done():
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		log.Printf("SSE stream read error: %v", err)
	}
}
