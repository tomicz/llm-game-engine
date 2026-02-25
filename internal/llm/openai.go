package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const openAIBaseURL = "https://api.openai.com/v1/chat/completions"

// OpenAI implements Client using the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey string
	client *http.Client
}

// NewOpenAI returns a Client that uses the OpenAI API with the given API key.
func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{
		apiKey: apiKey,
		client: http.DefaultClient,
	}
}

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Complete sends system and user messages to the OpenAI API and returns the assistant reply.
func (c *OpenAI) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("openai: API key not set")
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIBaseURL, bytes.NewReader(body))
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
		return "", fmt.Errorf("openai: %s", resp.Status)
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}
