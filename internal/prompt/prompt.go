package prompt

import (
	_ "embed"
	"strings"
)

//go:embed defaults/system-prompt.md
var defaultSystemPromptRaw string

//go:embed defaults/summary-prompt.md
var defaultSummaryPromptRaw string

// DefaultSystemPrompt is the built-in system prompt used when the user
// has not configured a custom one.
var DefaultSystemPrompt = strings.TrimSpace(defaultSystemPromptRaw)

// DefaultSummaryPrompt is the built-in system prompt for the summary model
// that generates conversation titles.
var DefaultSummaryPrompt = strings.TrimSpace(defaultSummaryPromptRaw)

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
