package ui

import "fmt"

// Inspector is a right-side panel that shows name, position, and scale of a selected object.
// It owns its nodes and updates their text when AppendNodes is called with visible true.
// Shown only when visible is true (e.g. terminal open and an object selected).
type Inspector struct {
	panel    *Node
	title    *Node
	name     *Node
	position *Node
	scale    *Node
}

// NewInspector creates an Inspector with nodes styled by the engine's CSS (.inspector, .inspector-title, etc.).
func NewInspector() *Inspector {
	return &Inspector{
		panel:    NewNode("panel", "inspector", "", ""),
		title:    NewNode("label", "inspector-title", "", "Inspector"),
		name:     NewNode("label", "inspector-name", "", ""),
		position: NewNode("label", "inspector-position", "", ""),
		scale:    NewNode("label", "inspector-scale", "", ""),
	}
}

// Selection holds the data shown in the inspector (name/type, position, scale).
// Pass this from the scene or game layer; ui does not depend on scene.
type Selection struct {
	Name     string
	Position [3]float32
	Scale    [3]float32
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
	return append(dst, in.panel, in.title, in.name, in.position, in.scale)
}
