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

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"` // llama-server format
					Reasoning        string `json:"reasoning"`         // Ollama format
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
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

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		log.Printf("SSE stream read error: %v", err)
	}
}
