package ui

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Node is a single UI element: panel, label, etc. It has optional class and id for CSS matching,
// bounds (position and size), and optional text for labels.
type Node struct {
	Type   string // "panel", "label", etc.
	Class  string // e.g. "menu" for .menu
	ID     string // e.g. "main" for #main
	Bounds rl.Rectangle
	Text   string // for label-type nodes
}

// NewNode creates a node with type and optional class, id, and text.
func NewNode(typ, class, id, text string) *Node {
	return &Node{
		Type:  typ,
		Class: class,
		ID:    id,
		Text:  text,
		Bounds: rl.Rectangle{X: 0, Y: 0, Width: 0, Height: 0},
	}
}
