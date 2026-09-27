// Package agent turns natural-language requests into game changes: an LLM replies with JSON
// actions, which are applied through registered handlers.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"game-engine/internal/commands"
	"game-engine/internal/llm"
)

// defaultModel is used when no model is configured.
const defaultModel = "gpt-4o-mini"

// Action is one action object from the model, e.g. {"action":"add_object","type":"cube",...}.
type Action map[string]any

// Handler applies one action. Handlers run on the main thread.
type Handler func(Action) error

// Agent sends requests to an LLM and applies the returned actions.
type Agent struct {
	client       llm.Client
	handlers     map[string]Handler
	systemPrompt string
}

// New returns an agent using client. The terminal commands in reg are described to the model and
// can be run with the run_cmd action (except Manual ones). Register other actions with
// RegisterHandler (see RegisterSceneHandlers).
func New(client llm.Client, reg *commands.Registry) *Agent {
	a := &Agent{
		client:       client,
		handlers:     make(map[string]Handler),
		systemPrompt: buildSystemPrompt(reg),
	}
	a.RegisterHandler("run_cmd", runCmdHandler(reg))
	return a
}

// RegisterHandler adds a handler for an action type (e.g. "add_object").
func (a *Agent) RegisterHandler(action string, h Handler) {
	a.handlers[action] = h
}

// Plan sends the request to the model and returns the actions it chose. viewContext, when
// non-empty, describes what the camera sees so the model can resolve phrases like "the one on the
// right". Plan does not touch game state and may be called from any goroutine.
func (a *Agent) Plan(ctx context.Context, model, userMessage, viewContext string) ([]Action, error) {
	if model == "" {
		model = defaultModel
	}
	prompt := userMessage
	if viewContext != "" {
		prompt = "Current camera view: " + viewContext + "\n\nUser: " + userMessage
	}
	reply, err := a.client.Complete(ctx, model, a.systemPrompt, prompt)
	if err != nil {
		return nil, err
	}
	actions, err := parseActions(reply)
	if err != nil {
		return nil, fmt.Errorf("LLM response invalid: %w", err)
	}
	return actions, nil
}

// Apply runs each action through its handler and returns a summary for the terminal. A failing
// action does not stop the rest. Must be called on the main thread.
func (a *Agent) Apply(actions []Action) string {
	applied := 0
	var problems []string
	for i, act := range actions {
		name, _ := act["action"].(string)
		h, ok := a.handlers[name]
		switch {
		case name == "":
			problems = append(problems, fmt.Sprintf("action %d: missing action", i+1))
		case !ok:
			problems = append(problems, fmt.Sprintf("action %d: unknown action %q", i+1, name))
		default:
			if err := h(act); err != nil {
				problems = append(problems, fmt.Sprintf("action %d (%s): %v", i+1, name, err))
			} else {
				applied++
			}
		}
	}
	switch {
	case len(problems) > 0:
		return strings.Join(problems, "; ")
	case applied > 0:
		return fmt.Sprintf("Done. Applied %d action(s).", applied)
	default:
		return "No actions to apply."
	}
}

// runCmdHandler runs {"action":"run_cmd","args":[...]} through the command registry.
func runCmdHandler(reg *commands.Registry) Handler {
	return func(act Action) error {
		raw, ok := act["args"].([]any)
		if !ok || len(raw) == 0 {
			return errors.New("missing or empty args")
		}
		args := make([]string, len(raw))
		for i, v := range raw {
			s, ok := v.(string)
			if !ok {
				return errors.New("args must be strings")
			}
			args[i] = s
		}
		if c, ok := reg.Lookup(args[0]); ok && c.Manual {
			return fmt.Errorf("%s can only be changed manually (use cmd %s)", c.Name, c.Name)
		}
		return reg.Execute(args)
	}
}

// parseActions extracts the actions from a model reply. It tolerates markdown fences and text
// around the JSON, an "actions" object instead of an array, and a single top-level action.
func parseActions(reply string) ([]Action, error) {
	start := strings.IndexByte(reply, '{')
	if start < 0 {
		return nil, errors.New("no JSON object in response")
	}
	// Decode only the first JSON value; anything after it (e.g. a closing ``` fence) is ignored.
	var raw map[string]any
	if err := json.NewDecoder(strings.NewReader(reply[start:])).Decode(&raw); err != nil {
		return nil, err
	}
	switch v := raw["actions"].(type) {
	case []any:
		out := make([]Action, len(v))
		for i, item := range v {
			out[i], _ = item.(map[string]any) // non-objects become empty and are reported by Apply
		}
		return out, nil
	case map[string]any:
		return []Action{v}, nil
	}
	if _, ok := raw["action"]; ok {
		return []Action{raw}, nil
	}
	return nil, errors.New(`missing actions array (reply had no "actions" or "action" object)`)
}
