package scene

import (
	"errors"
	"slices"
)

// undoOp is one recorded change: an object added (added != 0) or removed from index.
type undoOp struct {
	added   ObjectID
	removed *Object
	index   int
}

// undoLog keeps one level of undo: the adds and removals of the last scene-changing action.
// Changes made inside Scene.Group form a single step.
type undoLog struct {
	last    []undoOp
	pending []undoOp
	depth   int
}

func (u *undoLog) record(op undoOp) {
	u.pending = append(u.pending, op)
	if u.depth == 0 {
		u.commit()
	}
}

func (u *undoLog) commit() {
	if len(u.pending) > 0 {
		u.last, u.pending = u.pending, nil
	}
}

// Group runs fn and records every add and delete it makes as a single undo step, so e.g. one
// natural-language request or one template spawn is undone at once. Groups may nest.
func (s *Scene) Group(fn func()) {
	s.undo.depth++
	defer func() {
		s.undo.depth--
		if s.undo.depth == 0 {
			s.undo.commit()
		}
	}()
	fn()
}

// Undo reverts the last add or delete step.
func (s *Scene) Undo() error {
	ops := s.undo.last
	if len(ops) == 0 {
		return errors.New("nothing to undo")
	}
	s.undo.last = nil
	for _, op := range slices.Backward(ops) {
		if op.removed != nil {
			o := op.removed
			s.objects = slices.Insert(s.objects, min(op.index, len(s.objects)), o)
			continue
		}
		if i := s.indexOf(op.added); i >= 0 {
			s.removeAt(i)
		}
	}
	return nil
}
