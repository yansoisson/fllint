package prompt

import (
	_ "embed"
	"strings"
)

// PromptContext holds dynamic information injected into the system prompt.
type PromptContext struct {
	CurrentDateTime  string // e.g. "Monday, March 21, 2026 at 2:30 PM"
	WebSearchEnabled bool
}

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

// previousDefaults lists all previous versions of the default system prompt.
// When a user's saved prompt matches one of these, it is treated as "not customized"
// and automatically upgraded to the current default.
var previousDefaults = []string{
	// v0.1.0 – v0.6.2
	"You are Fllint, a helpful, accurate, and concise AI assistant running entirely on the user's local machine. No data leaves this device. Be direct and clear in your responses. When you don't know something, say so. Format responses with markdown when it improves readability.",
}

// IsDefault returns true if the given prompt matches the current default
// or any known previous default.
func IsDefault(content string) bool {
	content = strings.TrimSpace(content)
	if content == DefaultSystemPrompt {
		return true
	}
	for _, prev := range previousDefaults {
		if content == prev {
			return true
		}
	}
	return false
}

// Build composes the final system prompt from the configured system prompt
// and optional custom instructions.
//
// If systemPrompt is empty, the DefaultSystemPrompt is used.
// If customInstructions is non-empty, it is appended.
func Build(systemPrompt string, customInstructions string) string {
	return BuildWithContext(systemPrompt, customInstructions, PromptContext{})
}

// BuildWithContext composes the final system prompt with dynamic context injected.
func BuildWithContext(systemPrompt string, customInstructions string, ctx PromptContext) string {
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	// Append dynamic context
	var dynamic []string
	if ctx.CurrentDateTime != "" {
		dynamic = append(dynamic, "Current date and time: "+ctx.CurrentDateTime+".")
	}
	if ctx.WebSearchEnabled {
		dynamic = append(dynamic, "You have access to web search tools. When the user asks about current events, recent news, or anything requiring up-to-date information, use the web_search tool to find current information.")
	} else {
		dynamic = append(dynamic, "You do not have access to the internet or web search. If asked about very recent events, let the user know your knowledge has a cutoff date.")
	}
	if len(dynamic) > 0 {
		systemPrompt += "\n\n" + strings.Join(dynamic, "\n")
	}

	customInstructions = strings.TrimSpace(customInstructions)
	if customInstructions == "" {
		return systemPrompt
	}

	return systemPrompt + "\n\n" +
		"Additional instructions from the user:\n" + customInstructions
}
