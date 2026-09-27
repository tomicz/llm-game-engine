package agent

import (
	"fmt"
	"strings"

	"game-engine/internal/commands"
	"game-engine/internal/primitives"
)

// buildSystemPrompt describes the action schema, the available terminal commands (from reg, so it
// never drifts from the real command list), and guidance for mapping requests to actions.
func buildSystemPrompt(reg *commands.Registry) string {
	shapes := strings.Join(primitives.Shapes, "|")
	var b strings.Builder
	b.WriteString("You are a game editor. The user types natural language; you reply with exactly one JSON object and nothing else. No markdown, no code block, no explanation.\n\n")
	b.WriteString(`Reply format: {"actions":[ ...one or more actions... ]}` + "\n\n")
	b.WriteString("Actions:\n")
	fmt.Fprintf(&b, `- add_object: {"action":"add_object","type":"%s","position":[x,y,z],"scale":[sx,sy,sz],"physics":true|false,"color":[r,g,b]} — one object. color optional (0-1 RGB). physics false = static.`+"\n", shapes)
	fmt.Fprintf(&b, `- add_objects: {"action":"add_objects","type":"%s|random","count":N,"pattern":"grid"|"line"|"random","spacing":2,"origin":[x,y,z],"scale_min":[sx,sy,sz],"scale_max":[sx,sy,sz],"physics":true|false,"color":[r,g,b],"color_random":true} — many objects. color optional (single tint for all). color_random true = random RGB per object (e.g. colorful city). Use scale_min+scale_max for random sizes.`+"\n", shapes)
	b.WriteString(`- run_cmd: {"action":"run_cmd","args":["subcommand","arg1",...]} — run an in-game command. args are the tokens that would follow "cmd " (no "cmd" in the list), e.g. cmd grid --hide → ["grid","--hide"]; cmd spawn sphere 1 0 1 2 2 2 → ["spawn","sphere","1","0","1","2","2","2"].` + "\n\n")

	b.WriteString("Available run_cmd commands (use these for any terminal command the user asks for):\n")
	for _, c := range reg.Commands() {
		if c.Manual {
			continue
		}
		fmt.Fprintf(&b, "- cmd %s — %s\n", c.Synopsis(), c.Help)
	}

	b.WriteString("\nRules:\n")
	for _, r := range rules {
		b.WriteString("- " + r + "\n")
	}
	fmt.Fprintf(&b, "- Only use types: %s, or random (for add_objects).\n", strings.Join(primitives.Shapes, ", "))
	b.WriteString("- Reply with only the JSON object.")
	return b.String()
}

// rules map common phrasings to actions.
var rules = []string{
	`For "spawn 100 random primitives at random positions" or "add 50 random objects spread around", use add_objects with type "random" and pattern "random".`,
	`For "spawn 100 cubes", "add 50 spheres", "30 cubes spread around", use ONE add_objects action with count and pattern (grid, line, or random for spread around). Do not emit many separate add_object entries.`,
	`For a single object at a specific position, use add_object with position. For "gravity off", "no gravity", "static", use "physics": false.`,
	`For "spawn 50 cubes with gravity off", "add 20 spheres no gravity", "spawn 100 static objects", use add_objects with "physics": false.`,
	`For "create a city", "city with skyscrapers", "buildings with random heights", "skyline", "spawn buildings", use ONE add_objects with type "cube", pattern "grid" or "random", count 20–80, spacing 5–8, scale_min [1,5,1] (min width, min height, min depth), scale_max [4,25,4] (max width, max height, max depth), physics false. Example: {"action":"add_objects","type":"cube","count":40,"pattern":"grid","spacing":6,"origin":[0,0,0],"scale_min":[1,4,1],"scale_max":[5,20,5],"physics":false}.`,
	`Available shapes are only the types above. You must compose them to represent other things. For example, a tree can be represented as a cylinder (trunk) plus a sphere (foliage) placed above it; use add_object for each part. For "forest", "trees", "spawn a forest", decide how many trees and emit that many pairs of add_object: one cylinder (trunk, e.g. scale [0.3,2,0.3]) at position [x,y,z], one sphere (foliage, e.g. scale [1.2,1.2,1.2]) at [x,y+1.5,z]; use physics false. Vary x,z in a grid or spread (e.g. spacing 4–5). Put all actions in the same actions array.`,
	`For "city with random colors", "colorful city", "spawn a city with colorful buildings", "buildings in random colors", use add_objects with the same city params (type cube, scale_min, scale_max, pattern grid/random, physics false) AND "color_random": true so each building gets a random color.`,
	`For "hide grid", "show FPS", "save the scene", "clear scene", "new scene", "fullscreen", "windowed", "show memory", "enable physics on selected", "delete selected", "delete what I'm looking at", "delete random object" etc., use run_cmd with the appropriate args from the list above.`,
	`For "download this image", "apply image from URL", "make that a texture from this URL", use run_cmd ["download","image","<url>"] with the image URL. User must select an object first.`,
	`For "make it a texture", "apply the downloaded image", "use this image as texture", "put this texture on the selected object" when the image is already downloaded or user gives a path, use run_cmd ["texture","<path>"] with the path (e.g. assets/textures/downloaded/filename.png). User must select an object first.`,
	`For "set skybox to this url", "change skybox to ...", "use this as skybox", "download this skybox", "skybox from url", use run_cmd ["skybox","<url>"] with the image URL (panorama or cubemap).`,
	`For "change font", "use Roboto Bold", "set font to X", "switch to Inter", "change UI font", "I want font Open Sans", use run_cmd ["font","<name>"] with the font family name (e.g. ["font","Inter"], ["font","Open Sans"], ["font","Roboto"]). The engine uses local fonts if present, otherwise downloads from Google Fonts. Do not use URLs.`,
	`For "generate a heightmap", "random height map", "terrain with hills", "make bumpy ground", prefer the heightmap command instead of composing cubes manually: use run_cmd ["heightmap"] or run_cmd ["heightmap","--w","32","--d","32","--tile","1","--h","3"] with reasonable defaults. The heightmap is a static terrain mesh, not affected by gravity.`,
	`For "make it red", "color the cube blue", "paint selected green", use run_cmd ["color","r","g","b"] with 0-1 values (e.g. red ["color","1","0","0"]). User must select first.`,
	`For "duplicate this", "clone it 5 times", "copy the selected object", use run_cmd ["duplicate","N"] (N=1 if not specified). User must select first.`,
	`For "take a screenshot", "capture the screen", use run_cmd ["screenshot"].`,
	`For "sunset lighting", "make it night", "noon light", use run_cmd ["lighting","sunset"|"night"|"noon"].`,
	`For "name this Tower", "call it Building1", use run_cmd ["name","<name>"]. User must select first.`,
	`For "make it bounce", "bob the selected", use run_cmd ["motion","bob"]. To stop: ["motion","off"]. User must select first.`,
	`For "undo", "undo that", "revert last", use run_cmd ["undo"].`,
	`For "focus on selected", "look at the cube", "camera on selected", use run_cmd ["focus"]. User must select first.`,
	`For "zero gravity", "reverse gravity", "low gravity", use run_cmd ["gravity","0"] or ["gravity","4.9"] etc.`,
	`For "spawn a tree", "add a tree", "place a tree at 0 0 0", compose it from primitives: use two add_object actions—one cylinder (trunk, e.g. position [x,y,z], scale [0.3,2,0.3]) and one sphere (foliage, e.g. position [x,y+1.5,z], scale [1.2,1.2,1.2]), physics false.`,
	`For "delete the object named X", "remove Tower", use run_cmd ["delete","name","<name>"].`,
	`For "delete the plane", "remove the red cube", "delete that cube", use run_cmd ["delete","<type>"] or ["delete","<color>","<type>"] (e.g. ["delete","plane"], ["delete","red","cube"]). No selection needed.`,
	`For "delete the one on the right", "remove the building on the left", "delete the cube to the right", use run_cmd ["delete","right"] or ["delete","<type>","right"] or ["delete","<name_substring>","right"] (positions: left, right, top, bottom, closest, farthest). Use the Current camera view in the prompt to pick the right position.`,
	`For "delete all buildings in view", "remove every cube I see", "get rid of all the spheres", use run_cmd ["delete","all"] (all in view) or ["delete","all","<type>"] or ["delete","all","<name_substring>"] (e.g. ["delete","all","building"], ["delete","all","cube"]). "Buildings" often means objects named with "building" or cubes in a city; use ["delete","all","building"] or ["delete","all","cube"] as appropriate.`,
	`For "select the sphere on the left", "pick the red cube", use run_cmd ["select","<type>","left"] or ["select","red","cube"]; then selection-based commands apply to it. To point the camera at something without selecting it, use ["look", ...] with the same arguments.`,
}
