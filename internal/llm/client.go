// Package llm has minimal chat-completion clients for OpenAI-compatible APIs and Ollama.
package llm

import "context"

// Client sends a prompt to an LLM and returns the reply text.
// Model is provider-specific (e.g. "gpt-4o-mini", "qwen3-coder:30b"). Implementations must be
// safe to call from any goroutine and honor ctx cancellation.
type Client interface {
	Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error)
}

// message is one chat message, shared by the OpenAI and Ollama request formats.
type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
