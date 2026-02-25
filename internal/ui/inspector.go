package ui

import "fmt"

// Inspector is a right-side panel that shows name, position, scale, and physics of a selected object.
// It owns its nodes and updates their text when AppendNodes is called with visible true.
// Shown only when visible is true (e.g. terminal open and an object selected).
type Inspector struct {
	panel    *Node
	title    *Node
	name     *Node
	position *Node
	scale    *Node
	physics  *Node
	texture  *Node
}

// NewInspector creates an Inspector with nodes styled by the engine's CSS (.inspector, .inspector-title, etc.).
func NewInspector() *Inspector {
	return &Inspector{
		panel:    NewNode("panel", "inspector", "", ""),
		title:    NewNode("label", "inspector-title", "", "Inspector"),
		name:     NewNode("label", "inspector-name", "", ""),
		position: NewNode("label", "inspector-position", "", ""),
		scale:    NewNode("label", "inspector-scale", "", ""),
		physics:  NewNode("label", "inspector-physics", "", ""),
		texture:  NewNode("label", "inspector-texture", "", ""),
	}
}

// Selection holds the data shown in the inspector (name/type, position, scale, physics, texture).
// Pass this from the scene or game layer; ui does not depend on scene.
type Selection struct {
	Name     string
	Position [3]float32
	Scale    [3]float32
	Physics  bool   // true = falling/collision on; false = static (use cmd physics on/off to toggle)
	Texture  string // path to texture if set (e.g. assets/textures/downloaded/foo.png)
}

// AppendNodes appends inspector nodes to dst when visible is true, after updating labels from sel.
// When visible is false, dst is returned unchanged. Call every frame so visibility and content stay in sync.
func (in *Inspector) AppendNodes(dst []*Node, visible bool, sel Selection) []*Node {
	if !visible {
		return dst
	}
	in.title.Text = "Inspector"
	in.name.Text = "Name: " + sel.Name
	in.position.Text = fmt.Sprintf("Position: %.2f, %.2f, %.2f", sel.Position[0], sel.Position[1], sel.Position[2])
	in.scale.Text = fmt.Sprintf("Scale: %.2f, %.2f, %.2f", sel.Scale[0], sel.Scale[1], sel.Scale[2])
	if sel.Physics {
		in.physics.Text = "Physics: On"
	} else {
		in.physics.Text = "Physics: Off"
	}
	if sel.Texture != "" {
		in.texture.Text = "Texture: " + sel.Texture
	} else {
		in.texture.Text = "Texture: —"
	}
	return append(dst, in.panel, in.title, in.name, in.position, in.scale, in.physics, in.texture)
}
