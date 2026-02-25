package physics

// Body is a 3D rigid body with position, velocity, and AABB (from scale).
// Used for dynamic or static objects; static bodies do not move and are not affected by gravity.
type Body struct {
	Position [3]float32
	Velocity [3]float32
	Scale    [3]float32
	Mass     float32
	Static   bool
}

// NewBody returns a body with the given position and scale. Velocity is zero.
// mass is used for collision response; use 1 for default. Static bodies ignore gravity and velocity.
func NewBody(position, scale [3]float32, mass float32, static bool) *Body {
	if mass <= 0 {
		mass = 1
	}
	return &Body{
		Position: position,
		Velocity: [3]float32{0, 0, 0},
		Scale:    scale,
		Mass:     mass,
		Static:   static,
	}
}
