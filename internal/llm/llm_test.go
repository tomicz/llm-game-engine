package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func wantMessages(t *testing.T, got []message, system, user string) {
	t.Helper()
	want := []message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	if len(got) != len(want) {
		t.Fatalf("messages = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("messages[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOpenAICompatComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		var req openAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "gpt-test" {
			t.Errorf("model = %q, want gpt-test", req.Model)
		}
		wantMessages(t, req.Messages, "sys prompt", "hello")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi there"}},{"message":{"role":"assistant","content":"second"}}]}`))
	}))
	defer srv.Close()

	c := NewOpenAICompat("test", srv.URL, "secret")
	got, err := c.Complete(context.Background(), "gpt-test", "sys prompt", "hello")
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if got != "hi there" {
		t.Errorf("reply = %q, want first choice %q", got, "hi there")
	}
}

func TestOpenAICompatErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"non-200", http.StatusInternalServerError, `{}`, "test: 500"},
		{"unauthorized", http.StatusUnauthorized, `{}`, "test: 401"},
		{"empty choices", http.StatusOK, `{"choices":[]}`, "no choices"},
		{"invalid json", http.StatusOK, `not json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := NewOpenAICompat("test", srv.URL, "k")
			_, err := c.Complete(context.Background(), "m", "s", "u")
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestOpenAICompatMissingKey(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewOpenAICompat("test", srv.URL, "")
	_, err := c.Complete(context.Background(), "m", "s", "u")
	if err == nil || !strings.Contains(err.Error(), "API key not set") {
		t.Errorf("error = %v, want API key not set", err)
	}
	if called {
		t.Error("request sent despite missing API key")
	}
}

func TestOpenAICompatContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewOpenAICompat("test", srv.URL, "k")
	if _, err := c.Complete(ctx, "m", "s", "u"); err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestNewOllamaBaseURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", DefaultOllamaBaseURL},
		{"http://host:1234", "http://host:1234"},
		{"http://host:1234/", "http://host:1234"},
	}
	for _, tt := range tests {
		if got := NewOllama(tt.in).baseURL; got != tt.want {
			t.Errorf("NewOllama(%q).baseURL = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOllamaComplete(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantModel string
	}{
		{"explicit model", "llama3", "llama3"},
		{"empty model uses default", "", "qwen2.5-coder"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
					t.Errorf("request = %s %s, want POST /api/chat", r.Method, r.URL.Path)
				}
				var req ollamaChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("decode request: %v", err)
				}
				if req.Model != tt.wantModel {
					t.Errorf("model = %q, want %q", req.Model, tt.wantModel)
				}
				if req.Stream {
					t.Error("stream = true, want false")
				}
				wantMessages(t, req.Messages, "sys", "hello")
				w.Write([]byte(`{"message":{"role":"assistant","content":"ok"}}`))
			}))
			defer srv.Close()

			c := NewOllama(srv.URL + "/")
			got, err := c.Complete(context.Background(), tt.model, "sys", "hello")
			if err != nil {
				t.Fatalf("Complete error: %v", err)
			}
			if got != "ok" {
				t.Errorf("reply = %q, want ok", got)
			}
		})
	}
}

func TestOllamaErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr []string
	}{
		{"not found suggests pull", http.StatusNotFound, "", []string{"ollama", "404", "ollama pull mymodel"}},
		{"server error", http.StatusInternalServerError, "", []string{"ollama:", "500"}},
		{"invalid json", http.StatusOK, "nope", []string{"ollama:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := NewOllama(srv.URL).Complete(context.Background(), "mymodel", "s", "u")
			if err == nil {
				t.Fatal("expected error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}
