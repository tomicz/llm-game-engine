package llm

import "context"

// Client sends a prompt to an LLM and returns the reply text.
// Model is provider-specific (e.g. "gpt-4o-mini", "claude-3-haiku").
type Client interface {
	Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error)
}
