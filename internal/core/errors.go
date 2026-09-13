package core

import (
	"fmt"
)

// DbmError wraps domain-specific errors with context.
type DbmError struct {
	Op      string // Operation (e.g. "Connect", "Inspect", "StreamCopy")
	Engine  EngineType
	Message string
	Err     error
}

func (e *DbmError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s/%s] %s: %v", e.Engine, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s/%s] %s", e.Engine, e.Op, e.Message)
}

func (e *DbmError) Unwrap() error {
	return e.Err
}

// NewError creates a new DbmError.
func NewError(engine EngineType, op, message string, err error) error {
	return &DbmError{
		Engine:  engine,
		Op:      op,
		Message: message,
		Err:     err,
	}
}
