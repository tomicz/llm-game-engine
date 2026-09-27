package scene

import "log"

// ViewAwareness reports objects entering and leaving the camera view. Attach it with
// EnableViewAwareness; it is updated every game-mode frame.
type ViewAwareness struct {
	// OnEnterView is called when an object comes into view (optional).
	OnEnterView func(obj *Object, distance float32)
	// OnLeaveView is called when a still-existing object leaves the view (optional).
	OnLeaveView func(obj *Object)

	lastVisible map[ObjectID]struct{} // nil until the first update
}

// NewViewAwarenessWithLogging returns a ViewAwareness that logs enter/leave events.
func NewViewAwarenessWithLogging() *ViewAwareness {
	return &ViewAwareness{
		OnEnterView: func(obj *Object, distance float32) {
			log.Printf("[camera] enter view: %s (%s) at %.2f", obj.Label(), obj.Type, distance)
		},
		OnLeaveView: func(obj *Object) {
			log.Printf("[camera] leave view: %s (%s)", obj.Label(), obj.Type)
		},
	}
}

// EnableViewAwareness attaches a (or detaches with nil).
func (s *Scene) EnableViewAwareness(a *ViewAwareness) {
	s.awareness = a
}

func (s *Scene) updateViewAwareness() {
	a := s.awareness
	if a == nil {
		return
	}
	visible := s.ObjectsInView()
	cur := make(map[ObjectID]struct{}, len(visible))
	for _, v := range visible {
		cur[v.Object.ID] = struct{}{}
		if _, was := a.lastVisible[v.Object.ID]; a.lastVisible != nil && !was && a.OnEnterView != nil {
			a.OnEnterView(v.Object, v.Distance)
		}
	}
	for id := range a.lastVisible {
		if _, now := cur[id]; now || a.OnLeaveView == nil {
			continue
		}
		if o, ok := s.Object(id); ok {
			a.OnLeaveView(o)
		}
	}
	a.lastVisible = cur
}
