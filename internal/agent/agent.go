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
		"- add_object: {\"action\":\"add_object\",\"type\":\"cube|sphere|cylinder|plane\",\"position\":[x,y,z],\"scale\":[sx,sy,sz],\"physics\":true|false,\"color\":[r,g,b]} — one object. color optional (0-1 RGB). physics false = static.\n" +
		"- add_objects: {\"action\":\"add_objects\",\"type\":\"cube|sphere|cylinder|plane|random\",\"count\":N,\"pattern\":\"grid\"|\"line\"|\"random\",\"spacing\":2,\"origin\":[x,y,z],\"scale_min\":[sx,sy,sz],\"scale_max\":[sx,sy,sz],\"physics\":true|false,\"color\":[r,g,b],\"color_random\":true} — many objects. color optional (single tint for all). color_random true = random RGB per object (e.g. colorful city). Use scale_min+scale_max for random sizes.\n" +
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
		"- delete: remove object → [\"delete\",\"selected\"] | [\"delete\",\"look\"] | [\"delete\",\"random\"] | [\"delete\",\"name\",\"<name>\"]\n" +
		"- color: set selected object RGB (0-1) → [\"color\",\"1\",\"0\",\"0\"] for red (user must select first)\n" +
		"- duplicate: clone selected N times → [\"duplicate\",\"5\"] (user must select first)\n" +
		"- screenshot: capture view → [\"screenshot\"]\n" +
		"- lighting: time of day → [\"lighting\",\"noon\"] | [\"lighting\",\"sunset\"] | [\"lighting\",\"night\"]\n" +
		"- name: set selected object name → [\"name\",\"Tower\"] (user must select first)\n" +
		"- motion: set selected motion → [\"motion\",\"bob\"] | [\"motion\",\"off\"] (user must select first)\n" +
		"- undo: revert last add or delete → [\"undo\"]\n" +
		"- focus: point camera at selected → [\"focus\"] (user must select first)\n" +
		"- gravity: set gravity Y → [\"gravity\",\"-9.8\"] or [\"gravity\",\"0\"] for zero-g\n" +
		"- template: spawn preset → [\"template\",\"tree\"] or [\"template\",\"tree\",\"x\",\"y\",\"z\"]\n" +
		"- download: download image from URL and apply as texture to selected object → [\"download\",\"image\",\"https://example.com/image.png\"] (user must select an object first)\n" +
		"- texture: apply image file as texture to selected object → [\"texture\",\"<path>\"] e.g. [\"texture\",\"assets/textures/downloaded/foo.png\"] (user must select an object first)\n" +
		"- skybox: set skybox from image URL (downloads in background, supports panorama/cubemap) → [\"skybox\",\"<url>\"] e.g. [\"skybox\",\"https://example.com/panorama.jpg\"]\n" +
		"- font: set UI font by name (e.g. Inter, Roboto, Open Sans). If the font is in assets/fonts/, it is used; otherwise the engine downloads it from Google Fonts (safe, no user URLs). → [\"font\",\"<name>\"] e.g. [\"font\",\"Inter\"] or [\"font\",\"Open Sans\"].\n\n" +
		"Rules:\n" +
		"- For \"spawn 100 random primitives at random positions\" or \"add 50 random objects spread around\", use add_objects with type \"random\" and pattern \"random\".\n" +
		"- For \"spawn 100 cubes\", \"add 50 spheres\", \"30 cubes spread around\", use ONE add_objects action with count and pattern (grid, line, or random for spread around). Do not emit many separate add_object entries.\n" +
		"- For a single object at a specific position, use add_object with position. For \"gravity off\", \"no gravity\", \"static\", use \"physics\": false.\n" +
		"- For \"spawn 50 cubes with gravity off\", \"add 20 spheres no gravity\", \"spawn 100 static objects\", use add_objects with \"physics\": false.\n" +
		"- For \"create a city\", \"city with skyscrapers\", \"buildings with random heights\", \"skyline\", \"spawn buildings\", use ONE add_objects with type \"cube\", pattern \"grid\" or \"random\", count 20–80, spacing 5–8, scale_min [1,5,1] (min width, min height, min depth), scale_max [4,25,4] (max width, max height, max depth), physics false. Example: {\"action\":\"add_objects\",\"type\":\"cube\",\"count\":40,\"pattern\":\"grid\",\"spacing\":6,\"origin\":[0,0,0],\"scale_min\":[1,4,1],\"scale_max\":[5,20,5],\"physics\":false}.\n" +
		"- Available shapes are only: cube, sphere, cylinder, plane. You must compose them to represent other things. For example, a tree can be represented as a cylinder (trunk) plus a sphere (foliage) placed above it; use add_object for each part. For \"forest\", \"trees\", \"spawn a forest\", decide how many trees and emit that many pairs of add_object: one cylinder (trunk, e.g. scale [0.3,2,0.3]) at position [x,y,z], one sphere (foliage, e.g. scale [1.2,1.2,1.2]) at [x,y+1.5,z]; use physics false. Vary x,z in a grid or spread (e.g. spacing 4–5). Put all actions in the same actions array.\n" +
		"- For \"city with random colors\", \"colorful city\", \"spawn a city with colorful buildings\", \"buildings in random colors\", use add_objects with the same city params (type cube, scale_min, scale_max, pattern grid/random, physics false) AND \"color_random\": true so each building gets a random color.\n" +
		"- For \"hide grid\", \"show FPS\", \"save the scene\", \"clear scene\", \"new scene\", \"fullscreen\", \"windowed\", \"show memory\", \"set model to X\", \"enable physics on selected\", \"delete selected\", \"delete what I'm looking at\", \"delete random object\" etc., use run_cmd with the appropriate args from the list above.\n" +
		"- For \"download this image\", \"apply image from URL\", \"make that a texture from this URL\", use run_cmd [\"download\",\"image\",\"<url>\"] with the image URL. User must select an object first.\n" +
		"- For \"make it a texture\", \"apply the downloaded image\", \"use this image as texture\", \"put this texture on the selected object\" when the image is already downloaded or user gives a path, use run_cmd [\"texture\",\"<path>\"] with the path (e.g. assets/textures/downloaded/filename.png). User must select an object first.\n" +
		"- For \"set skybox to this url\", \"change skybox to ...\", \"use this as skybox\", \"download this skybox\", \"skybox from url\", use run_cmd [\"skybox\",\"<url>\"] with the image URL (panorama or cubemap).\n" +
		"- For \"change font\", \"use Roboto Bold\", \"set font to X\", \"switch to Inter\", \"change UI font\", \"I want font Open Sans\", use run_cmd [\"font\",\"<name>\"] with the font family name (e.g. [\"font\",\"Inter\"], [\"font\",\"Open Sans\"], [\"font\",\"Roboto\"]). The engine uses local fonts if present, otherwise downloads from Google Fonts. Do not use URLs.\n" +
		"- For \"make it red\", \"color the cube blue\", \"paint selected green\", use run_cmd [\"color\",\"r\",\"g\",\"b\"] with 0-1 values (e.g. red [\"color\",\"1\",\"0\",\"0\"]). User must select first.\n" +
		"- For \"duplicate this\", \"clone it 5 times\", \"copy the selected object\", use run_cmd [\"duplicate\",\"N\"] (N=1 if not specified). User must select first.\n" +
		"- For \"take a screenshot\", \"capture the screen\", use run_cmd [\"screenshot\"].\n" +
		"- For \"sunset lighting\", \"make it night\", \"noon light\", use run_cmd [\"lighting\",\"sunset\"|\"night\"|\"noon\"].\n" +
		"- For \"name this Tower\", \"call it Building1\", use run_cmd [\"name\",\"<name>\"]. User must select first.\n" +
		"- For \"make it bounce\", \"bob the selected\", use run_cmd [\"motion\",\"bob\"]. To stop: [\"motion\",\"off\"]. User must select first.\n" +
		"- For \"undo\", \"undo that\", \"revert last\", use run_cmd [\"undo\"].\n" +
		"- For \"focus on selected\", \"look at the cube\", \"camera on selected\", use run_cmd [\"focus\"]. User must select first.\n" +
		"- For \"zero gravity\", \"reverse gravity\", \"low gravity\", use run_cmd [\"gravity\",\"0\"] or [\"gravity\",\"4.9\"] etc.\n" +
		"- For \"spawn a tree\", \"add a tree\", \"place a tree at 0 0 0\", compose it from primitives: use two add_object actions—one cylinder (trunk, e.g. position [x,y,z], scale [0.3,2,0.3]) and one sphere (foliage, e.g. position [x,y+1.5,z], scale [1.2,1.2,1.2]), physics false.\n" +
		"- For \"delete the object named X\", \"remove Tower\", use run_cmd [\"delete\",\"name\",\"<name>\"].\n" +
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
