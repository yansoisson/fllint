package prompt

import "strings"

// DefaultSystemPrompt is the built-in system prompt used when the user
// has not configured a custom one.
const DefaultSystemPrompt = "You are Fllint, a helpful, accurate, and concise AI assistant " +
	"running entirely on the user's local machine. No data leaves this device. " +
	"Be direct and clear in your responses. When you don't know something, say so. " +
	"Format responses with markdown when it improves readability."

// Build composes the final system prompt from the configured system prompt
// and optional custom instructions.
//
// If systemPrompt is empty, the DefaultSystemPrompt is used.
// If customInstructions is non-empty, it is appended.
func Build(systemPrompt string, customInstructions string) string {
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	customInstructions = strings.TrimSpace(customInstructions)
	if customInstructions == "" {
		return systemPrompt
	}

	return systemPrompt + "\n\n" +
		"Additional instructions from the user:\n" + customInstructions
}
