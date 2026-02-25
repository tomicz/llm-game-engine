package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DefaultOllamaBaseURL is the default base URL for a local Ollama server.
const DefaultOllamaBaseURL = "http://localhost:11434"

// Ollama implements Client using the Ollama /api/chat endpoint (e.g. Qwen 3 Coder, Llama).
type Ollama struct {
	baseURL string
	client  *http.Client
}

// NewOllama returns a Client that uses the Ollama API at baseURL (e.g. http://localhost:11434).
// If baseURL is empty, DefaultOllamaBaseURL is used.
func NewOllama(baseURL string) *Ollama {
	u := strings.TrimSuffix(baseURL, "/")
	if u == "" {
		u = DefaultOllamaBaseURL
	}
	return &Ollama{
		baseURL: u,
		client:  http.DefaultClient,
	}
}

type ollamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type ollamaChatResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

// Complete sends system and user messages to Ollama and returns the assistant reply.
func (c *Ollama) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	if model == "" {
		model = "qwen2.5-coder"
	}
	reqBody := ollamaChatRequest{
		Model:  model,
		Stream: false,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	url := c.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: %s", resp.Status)
	}
	var out ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	return out.Message.Content, nil
}
