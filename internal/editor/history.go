package editor

import (
	"sync"

	"github.com/noorshikalgar/story/internal/session"
)

// editSnapshot captures a step's full editable surface — image bytes and
// annotation list together — as one unit. Redaction and annotation edits
// used to have two disconnected undo mechanisms (image-only undo/redo,
// and no history at all for annotations); snapshotting both together
// means a single Undo/Redo/Reset walks the true chronological order of
// everything the user did to a step, regardless of which kind of edit it
// was.
type editSnapshot struct {
	Image       []byte
	Annotations []session.Annotation
}

// cloneAnnotations copies the slice (session.Annotation has no reference
// fields, so a shallow element copy is a full copy) so a snapshot can't be
// mutated by later in-place edits to the live step's slice.
func cloneAnnotations(anns []session.Annotation) []session.Annotation {
	return append([]session.Annotation(nil), anns...)
}

// editHistory is in-memory, per-step undo/redo/reset state scoped to one
// running "story edit" process — not persisted. This is deliberately
// ephemeral (decision #17): the durable record of what changed is
// Session.EditLog, written to by every mutating session function
// regardless of history state; editHistory just makes "oops, undo that"
// pleasant while the editor is open.
type editHistory struct {
	mu       sync.Mutex
	baseline map[int]editSnapshot
	undo     map[int][]editSnapshot
	redo     map[int][]editSnapshot
}

func newEditHistory() *editHistory {
	return &editHistory{
		baseline: make(map[int]editSnapshot),
		undo:     make(map[int][]editSnapshot),
		redo:     make(map[int][]editSnapshot),
	}
}

// push records the state immediately before a mutation that's about to be
// applied. The first push seen for a step also seeds the Reset baseline.
// A fresh edit always clears redo history for that step, same as any
// other undo/redo system — once you make a new change, "redo" no longer
// has a coherent future to jump to.
func (h *editHistory) push(stepIndex int, before editSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.baseline[stepIndex]; !ok {
		h.baseline[stepIndex] = before
	}
	h.undo[stepIndex] = append(h.undo[stepIndex], before)
	delete(h.redo, stepIndex)
}

// undoStep pops the most recent pre-edit snapshot and returns it, pushing
// `current` onto the redo stack so a subsequent Redo can restore it.
func (h *editHistory) undoStep(stepIndex int, current editSnapshot) (editSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stack := h.undo[stepIndex]
	if len(stack) == 0 {
		return editSnapshot{}, false
	}
	prev := stack[len(stack)-1]
	h.undo[stepIndex] = stack[:len(stack)-1]
	h.redo[stepIndex] = append(h.redo[stepIndex], current)
	return prev, true
}

// redoStep pops the most recently undone snapshot and returns it, pushing
// `current` back onto the undo stack.
func (h *editHistory) redoStep(stepIndex int, current editSnapshot) (editSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stack := h.redo[stepIndex]
	if len(stack) == 0 {
		return editSnapshot{}, false
	}
	next := stack[len(stack)-1]
	h.redo[stepIndex] = stack[:len(stack)-1]
	h.undo[stepIndex] = append(h.undo[stepIndex], current)
	return next, true
}

// resetStep returns the step's very first recorded state (before any edit
// made during this editor session). Reset is itself pushed onto the undo
// stack rather than clearing history outright, so a reset can be undone
// like any other edit instead of being a dead end.
func (h *editHistory) resetStep(stepIndex int, current editSnapshot) (editSnapshot, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	base, ok := h.baseline[stepIndex]
	if !ok {
		return editSnapshot{}, false
	}
	h.undo[stepIndex] = append(h.undo[stepIndex], current)
	delete(h.redo, stepIndex)
	return base, true
}
