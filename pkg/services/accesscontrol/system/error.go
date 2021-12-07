package system

import "errors"

var (
	ErrInvalidActions    = errors.New("invalid actions")
	ErrInvalidAssignment = errors.New("invalid assignment")
)
