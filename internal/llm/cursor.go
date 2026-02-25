package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// Cursor API base URL. Cursor uses Basic auth (API key as username, empty password).
// If Cursor exposes an OpenAI-compatible chat endpoint, it may be under /v1/chat/completions.
const cursorBaseURL = "https://api.cursor.com/v1/chat/completions"

// Cursor implements Client using the Cursor API with Basic authentication.
type Cursor struct {
	apiKey string
	client *http.Client
}

// NewCursor returns a Client that uses the Cursor API with the given API key.
func NewCursor(apiKey string) *Cursor {
	return &Cursor{
		apiKey: apiKey,
		client: http.DefaultClient,
	}
}

// Complete sends system and user messages to the Cursor API and returns the assistant reply.
// Uses the same request/response shape as OpenAI (model, messages, choices[].message.content).
func (c *Cursor) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("cursor: API key not set")
	}
	reqBody := openAIRequest{
		Model: model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cursorBaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.apiKey+":")))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("cursor: 404 — Cursor API does not expose a chat completion endpoint at this URL. For natural-language commands, set OPENAI_API_KEY in .env")
		}
		return "", fmt.Errorf("cursor: %s", resp.Status)
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("cursor: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}
