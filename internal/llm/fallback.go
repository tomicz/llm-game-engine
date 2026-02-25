package llm

import "context"

// Fallback tries primary first; if it returns an error, tries secondary.
// Use when primary (e.g. Cursor) may not expose the endpoint but secondary (e.g. OpenAI) does.
type Fallback struct {
	Primary   Client
	Secondary Client
}

// Complete calls Primary.Complete; on any error, calls Secondary.Complete.
func (f *Fallback) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	s, err := f.Primary.Complete(ctx, model, systemPrompt, userMessage)
	if err != nil && f.Secondary != nil {
		return f.Secondary.Complete(ctx, model, systemPrompt, userMessage)
	}
	return s, err
}
