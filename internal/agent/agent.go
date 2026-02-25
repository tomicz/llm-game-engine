package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"game-engine/internal/llm"
)

// Handler applies one action. Payload is the action object (e.g. {"action":"add_object", "type":"cube", ...}).
// Returns an error to report to the user; the agent will still process remaining actions.
type Handler func(payload map[string]interface{}) error

// Agent turns natural language into game updates via an LLM and a registry of action handlers.
type Agent struct {
	client   llm.Client
	getModel func() string
	handlers map[string]Handler
}

// New returns an Agent that uses the given LLM client and model getter.
// Register handlers with RegisterHandler before calling Run.
func New(client llm.Client, getModel func() string) *Agent {
	return &Agent{
		client:   client,
		getModel: getModel,
		handlers: make(map[string]Handler),
	}
}

// RegisterHandler adds a handler for the given action type (e.g. "add_object", "run_cmd").
func (a *Agent) RegisterHandler(actionType string, h Handler) {
	a.handlers[actionType] = h
}

// Run sends the user message to the LLM, parses the JSON response, and applies each action.
// Returns a short summary for the terminal log, or an error.
func (a *Agent) Run(ctx context.Context, userMessage string) (summary string, err error) {
	model := a.getModel()
	if model == "" {
		model = "gpt-4o-mini"
	}
	systemPrompt := buildSystemPrompt()
	reply, err := a.client.Complete(ctx, model, systemPrompt, userMessage)
	if err != nil {
		return "", err
	}
	actions, parseErr := parseActions(reply)
	if parseErr != nil {
		return "", fmt.Errorf("LLM response invalid: %w", parseErr)
	}
	var applied int
	var messages []string
	for i, raw := range actions {
		payload, ok := raw.(map[string]interface{})
		if !ok {
			messages = append(messages, fmt.Sprintf("action %d: invalid object", i+1))
			continue
		}
		actionType, _ := payload["action"].(string)
		if actionType == "" {
			messages = append(messages, fmt.Sprintf("action %d: missing action", i+1))
			continue
		}
		h, ok := a.handlers[actionType]
		if !ok {
			messages = append(messages, fmt.Sprintf("action %d: unknown action %q", i+1, actionType))
			continue
		}
		if err := h(payload); err != nil {
			messages = append(messages, fmt.Sprintf("action %d (%s): %v", i+1, actionType, err))
			continue
		}
		applied++
	}
	if applied > 0 && len(messages) == 0 {
		return fmt.Sprintf("Done. Applied %d action(s).", applied), nil
	}
	if len(messages) > 0 {
		return strings.Join(messages, "; "), nil
	}
	return "No actions to apply.", nil
}

func buildSystemPrompt() string {
	return "You are a game editor. The user types natural language; you reply with exactly one JSON object and nothing else. No markdown, no code block, no explanation.\n\n" +
		"Schema:\n" +
		"- add_object: {\"action\":\"add_object\",\"type\":\"cube|sphere|cylinder|plane\",\"position\":[x,y,z],\"scale\":[sx,sy,sz],\"physics\":true|false} — one object. physics false = no gravity (static); omit or true = gravity on.\n" +
		"- add_objects: {\"action\":\"add_objects\",\"type\":\"cube|sphere|cylinder|plane|random\",\"count\":N,\"pattern\":\"grid\"|\"line\"|\"random\",\"spacing\":2,\"origin\":[x,y,z],\"scale\":[sx,sy,sz],\"scale_min\":[sx,sy,sz],\"scale_max\":[sx,sy,sz],\"physics\":true|false} — many objects. Use for \"spawn 100 cubes\", \"add 50 spheres\", \"30 cubes spread around\". type \"random\" = random primitive type. pattern \"random\" = random positions. Use scale_min and scale_max together for random size per object (e.g. buildings with random heights: scale_min [1,5,1], scale_max [4,25,4]). spacing and origin optional. physics false = no gravity (static).\n" +
		"- run_cmd: {\"action\":\"run_cmd\",\"args\":[\"subcommand\",\"arg1\",...]} — run an in-game command. Args are the tokens that would follow \"cmd \" (no \"cmd\" in the list).\n\n" +
		"Available run_cmd commands (use these for any terminal command the user asks for):\n" +
		"- grid: show/hide 3D editor grid → args [\"grid\",\"--show\"] or [\"grid\",\"--hide\"]\n" +
		"- fps: show/hide FPS counter → [\"fps\",\"--show\"] or [\"fps\",\"--hide\"]\n" +
		"- memalloc: show/hide memory usage → [\"memalloc\",\"--show\"] or [\"memalloc\",\"--hide\"]\n" +
		"- window: fullscreen/windowed → [\"window\",\"--fullscreen\"] or [\"window\",\"--windowed\"]\n" +
		"- spawn: add one primitive at position → [\"spawn\",\"cube\",\"0\",\"0\",\"0\"] or [\"spawn\",\"sphere\",\"1\",\"0\",\"1\",\"2\",\"2\",\"2\"] (type x y z [sx sy sz])\n" +
		"- save: save current scene to file → [\"save\"]\n" +
		"- newscene: clear all objects and save empty scene → [\"newscene\"]\n" +
		"- model: set AI model for future natural-language → [\"model\",\"llama-3.3-70b-versatile\"] or [\"model\",\"gpt-4o-mini\"]\n" +
		"- physics: enable/disable physics on selected object → [\"physics\",\"on\"] or [\"physics\",\"off\"] (user must select an object first)\n" +
		"- delete: remove an object → [\"delete\",\"selected\"] (selected object), [\"delete\",\"look\"] or [\"delete\",\"camera\"] (object camera is looking at), [\"delete\",\"random\"] (random object)\n" +
		"- download: download image from URL and apply as texture to selected object → [\"download\",\"image\",\"https://example.com/image.png\"] (user must select an object first)\n" +
		"- texture: apply image file as texture to selected object → [\"texture\",\"<path>\"] e.g. [\"texture\",\"assets/textures/downloaded/foo.png\"] (user must select an object first)\n" +
		"- skybox: set skybox from image URL (downloads in background, supports panorama/cubemap) → [\"skybox\",\"<url>\"] e.g. [\"skybox\",\"https://example.com/panorama.jpg\"]\n\n" +
		"Rules:\n" +
		"- For \"spawn 100 random primitives at random positions\" or \"add 50 random objects spread around\", use add_objects with type \"random\" and pattern \"random\".\n" +
		"- For \"spawn 100 cubes\", \"add 50 spheres\", \"30 cubes spread around\", use ONE add_objects action with count and pattern (grid, line, or random for spread around). Do not emit many separate add_object entries.\n" +
		"- For a single object at a specific position, use add_object with position. For \"gravity off\", \"no gravity\", \"static\", use \"physics\": false.\n" +
		"- For \"spawn 50 cubes with gravity off\", \"add 20 spheres no gravity\", \"spawn 100 static objects\", use add_objects with \"physics\": false.\n" +
		"- For \"create a city\", \"city with skyscrapers\", \"buildings with random heights\", \"skyline\", \"spawn buildings\", use ONE add_objects with type \"cube\", pattern \"grid\" or \"random\", count 20–80, spacing 5–8, scale_min [1,5,1] (min width, min height, min depth), scale_max [4,25,4] (max width, max height, max depth), physics false. Example: {\"action\":\"add_objects\",\"type\":\"cube\",\"count\":40,\"pattern\":\"grid\",\"spacing\":6,\"origin\":[0,0,0],\"scale_min\":[1,4,1],\"scale_max\":[5,20,5],\"physics\":false}.\n" +
		"- For \"hide grid\", \"show FPS\", \"save the scene\", \"clear scene\", \"new scene\", \"fullscreen\", \"windowed\", \"show memory\", \"set model to X\", \"enable physics on selected\", \"delete selected\", \"delete what I'm looking at\", \"delete random object\" etc., use run_cmd with the appropriate args from the list above.\n" +
		"- For \"download this image\", \"apply image from URL\", \"make that a texture from this URL\", use run_cmd [\"download\",\"image\",\"<url>\"] with the image URL. User must select an object first.\n" +
		"- For \"make it a texture\", \"apply the downloaded image\", \"use this image as texture\", \"put this texture on the selected object\" when the image is already downloaded or user gives a path, use run_cmd [\"texture\",\"<path>\"] with the path (e.g. assets/textures/downloaded/filename.png). User must select an object first.\n" +
		"- For \"set skybox to this url\", \"change skybox to ...\", \"use this as skybox\", \"download this skybox\", \"skybox from url\", use run_cmd [\"skybox\",\"<url>\"] with the image URL (panorama or cubemap).\n" +
		"- Only use types: cube, sphere, cylinder, plane, or random (for add_objects).\n" +
		"- Reply with only the JSON object."
}

// parseActions extracts the "actions" array from the LLM reply. Tolerates markdown, extra text, and single-action form.
func parseActions(reply string) ([]interface{}, error) {
	reply = strings.TrimSpace(reply)
	// Strip markdown code block if present
	if strings.HasPrefix(reply, "```") {
		reply = regexp.MustCompile("^```\\w*\\n?").ReplaceAllString(reply, "")
		reply = strings.TrimSuffix(reply, "```")
		reply = strings.TrimSpace(reply)
	}
	// Extract the first complete JSON object (in case there's text before/after)
	start := strings.Index(reply, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object in response")
	}
	reply = reply[start:]
	depth := 0
	end := -1
	for i, c := range reply {
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("unbalanced JSON braces")
	}
	reply = reply[:end]

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &raw); err != nil {
		return nil, err
	}
	// Prefer "actions" array
	if arr, ok := raw["actions"].([]interface{}); ok {
		return arr, nil
	}
	// Single object in "actions" (e.g. LLM returned {"actions": {...}})
	if obj, ok := raw["actions"].(map[string]interface{}); ok {
		return []interface{}{obj}, nil
	}
	// Top-level single action: {"action": "add_objects", ...}
	if _, hasAction := raw["action"]; hasAction {
		return []interface{}{raw}, nil
	}
	return nil, fmt.Errorf("missing actions array (reply had no \"actions\" or \"action\" object)")
}
