package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// AuthType controls how the API key is sent in the Authorization header.
type AuthType int

const (
	AuthBearer AuthType = iota // Authorization: Bearer <key>
	AuthBasic                  // Authorization: Basic base64(<key>:)
)

// Known provider base URLs.
const (
	OpenAIBaseURL  = "https://api.openai.com/v1/chat/completions"
	GroqBaseURL    = "https://api.groq.com/openai/v1/chat/completions"
	CursorBaseURL  = "https://api.cursor.com/v1/chat/completions"
)

// OpenAICompat implements Client for any OpenAI-compatible chat completions API.
type OpenAICompat struct {
	Name     string // provider name for error messages (e.g. "openai", "groq")
	BaseURL  string
	APIKey   string
	Auth     AuthType
	client   *http.Client
}

// NewOpenAICompat creates a client for an OpenAI-compatible API.
func NewOpenAICompat(name, baseURL, apiKey string, auth AuthType) *OpenAICompat {
	return &OpenAICompat{
		Name:    name,
		BaseURL: baseURL,
		APIKey:  apiKey,
		Auth:    auth,
		client:  http.DefaultClient,
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

func (c *OpenAICompat) Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	if c.APIKey == "" {
		return "", fmt.Errorf("%s: API key not set", c.Name)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	switch c.Auth {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	case AuthBasic:
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.APIKey+":")))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", c.Name, resp.Status)
	}
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: no choices in response", c.Name)
	}
	return out.Choices[0].Message.Content, nil
}
