package tui

import "errors"

var (
	// ErrAborted signals the user aborted input (e.g., Ctrl+C).
	ErrAborted = errors.New("tui: aborted")
	// ErrRepeatUnsupported is returned when the prompt driver cannot handle
	// repeat flows directly.
	ErrRepeatUnsupported = errors.New("tui: repeat prompt not supported")
	// ErrNotImplemented is a placeholder until the renderer gains full
	// interaction logic in later phases.
	ErrNotImplemented = errors.New("tui: render not implemented")
)
