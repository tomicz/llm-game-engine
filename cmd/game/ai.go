package main

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"game-engine/internal/agent"
	"game-engine/internal/commands"
	"game-engine/internal/llm"
)

// llmTimeout bounds one natural-language request. Local models can be slow to load.
const llmTimeout = 3 * time.Minute

// providers lists the supported LLM providers with their default models.
var providers = map[string]string{
	"groq":   "llama-3.3-70b-versatile",
	"openai": "gpt-4o-mini",
	"ollama": "qwen3-coder:30b",
}

// detectProvider picks a provider from the API keys in the environment: Groq, then OpenAI, then
// a local Ollama.
func detectProvider() string {
	switch {
	case os.Getenv("GROQ_API_KEY") != "":
		return "groq"
	case os.Getenv("OPENAI_API_KEY") != "":
		return "openai"
	default:
		return "ollama"
	}
}

// newLLMClient creates a client for provider using API keys from the environment.
func newLLMClient(provider string) (llm.Client, error) {
	switch provider {
	case "openai":
		return keyedClient("openai", llm.OpenAIBaseURL, "OPENAI_API_KEY")
	case "groq":
		return keyedClient("groq", llm.GroqBaseURL, "GROQ_API_KEY")
	case "ollama":
		return llm.NewOllama(os.Getenv("OLLAMA_BASE_URL")), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (use: ollama, openai, groq)", provider)
	}
}

func keyedClient(name, url, keyVar string) (llm.Client, error) {
	key := os.Getenv(keyVar)
	if key == "" {
		return nil, fmt.Errorf("%s not set in .env", keyVar)
	}
	return llm.NewOpenAICompat(name, url, key), nil
}

// setProvider switches the AI provider and model (empty = the provider's default) and rebuilds
// the agent. The choice is kept even if the client can't be created, so it is saved and shown.
func (a *App) setProvider(provider, model string) error {
	a.provider = provider
	a.model = cmp.Or(model, providers[provider])
	a.agent = nil
	client, err := newLLMClient(provider)
	if err != nil {
		return err
	}
	a.agent = agent.New(client, a.commands)
	agent.RegisterSceneHandlers(a.agent, a.scene)
	return nil
}

// handleNaturalLanguage sends a non-command terminal line to the AI agent in the background and
// applies the resulting actions on the main thread as one undo step.
func (a *App) handleNaturalLanguage(line string) {
	if a.agent == nil {
		a.log.Log(fmt.Sprintf("No AI available for provider %q. Add an API key to .env or start Ollama, then run cmd provider <name>.", a.provider))
		return
	}
	ag, model, view := a.agent, a.model, a.scene.ViewSummary()
	a.log.Log("Thinking…")
	background(a, func() ([]agent.Action, error) {
		ctx, cancel := context.WithTimeout(context.Background(), llmTimeout)
		defer cancel()
		return ag.Plan(ctx, model, line, view)
	}, func(actions []agent.Action, err error) {
		if err != nil {
			a.log.Log(err.Error())
			return
		}
		var summary string
		a.scene.Group(func() { summary = ag.Apply(actions) })
		a.log.Log(summary)
	})
}

func (a *App) registerAICommands() {
	a.commands.Register(commands.Command{
		Name: "provider", Usage: "[ollama | openai | groq]", Manual: true,
		Help: "Switch the AI provider (and reset to its default model). No argument shows the current one.",
		Run: func(args []string) error {
			if len(args) == 0 {
				a.log.Log(fmt.Sprintf("Current provider: %s (model: %s). Available: ollama, openai, groq", a.provider, a.model))
				return nil
			}
			name := strings.ToLower(args[0])
			if _, err := newLLMClient(name); err != nil {
				return err
			}
			_ = a.setProvider(name, "")
			a.savePrefs()
			a.log.Log(fmt.Sprintf("Switched to %s (model: %s)", name, a.model))
			return nil
		},
	})
	a.commands.Register(commands.Command{
		Name: "model", Usage: "[name]", Manual: true,
		Help: "Set the AI model for natural-language requests (e.g. gpt-4o-mini). No argument shows the current one.",
		Run: func(args []string) error {
			if len(args) == 0 {
				a.log.Log(fmt.Sprintf("Current model: %s (provider: %s)", a.model, a.provider))
				return nil
			}
			a.model = args[0]
			a.savePrefs()
			a.log.Log("Model set: " + a.model)
			return nil
		},
	})
}
