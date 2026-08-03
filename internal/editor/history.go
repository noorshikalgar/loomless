package editor

import "sync"

// imageHistory is an in-memory undo/redo stack of a step's screenshot
// bytes, scoped to one running "story edit" process — not persisted.
// Deliberately ephemeral: the permanent, durable record of what changed
// is Session.EditLog (every image_redact/image_highlight/image_undo/
// image_redo/image_reset entry, logged via session.LogEdit as each of
// these operations runs), which survives the editor closing; this is
// just enough state to make "oops, undo that" work while the editor is
// open, without keeping every past version of every screenshot around
// forever on disk.
type imageHistory struct {
	mu   sync.Mutex
	undo map[int][][]byte
	redo map[int][][]byte
}

func newImageHistory() *imageHistory {
	return &imageHistory{undo: map[int][][]byte{}, redo: map[int][][]byte{}}
}

// push records the image bytes as they were immediately before an edit
// that's about to be applied, and clears any redo history for that step
// — a fresh edit invalidates whatever had been undone.
func (h *imageHistory) push(stepIndex int, before []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.undo[stepIndex] = append(h.undo[stepIndex], before)
	h.redo[stepIndex] = nil
}

// undo returns the previous version and true if one exists, moving
// `current` onto the redo stack in the process.
func (h *imageHistory) undoStep(stepIndex int, current []byte) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stack := h.undo[stepIndex]
	if len(stack) == 0 {
		return nil, false
	}
	prev := stack[len(stack)-1]
	h.undo[stepIndex] = stack[:len(stack)-1]
	h.redo[stepIndex] = append(h.redo[stepIndex], current)
	return prev, true
}

// redo returns the next undone version and true if one exists, moving
// `current` back onto the undo stack.
func (h *imageHistory) redoStep(stepIndex int, current []byte) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stack := h.redo[stepIndex]
	if len(stack) == 0 {
		return nil, false
	}
	next := stack[len(stack)-1]
	h.redo[stepIndex] = stack[:len(stack)-1]
	h.undo[stepIndex] = append(h.undo[stepIndex], current)
	return next, true
}

// reset returns the earliest recorded version (the state before any edit
// made during this editor session) and true if there's any history at
// all, clearing both stacks for that step.
func (h *imageHistory) resetStep(stepIndex int) ([]byte, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	stack := h.undo[stepIndex]
	if len(stack) == 0 {
		return nil, false
	}
	original := stack[0]
	h.undo[stepIndex] = nil
	h.redo[stepIndex] = nil
	return original, true
}
