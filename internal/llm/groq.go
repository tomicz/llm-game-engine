package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const groqBaseURL = "https://api.groq.com/openai/v1/chat/completions"

// Groq implements Client using Groq's OpenAI-compatible Chat Completions API.
type Groq struct {
	apiKey string
	client *http.Client
}

// NewGroq returns a Client that uses the Groq API with the given API key.
func NewGroq(apiKey string) *Groq {
	return &Groq{
		apiKey: apiKey,
		client: http.DefaultClient,
	}
}

// Complete sends system and user messages to the Groq API and returns the assistant reply.
func (c *Groq) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("groq: API key not set")
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqBaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq: %s", resp.Status)
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("groq: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}
