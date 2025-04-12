package dto

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrInternalFailure = errors.New("internal failure")
	ErrBadRequest      = errors.New("bad request")
	ErrNotAuthorized   = errors.New("not authorized")
	ErrConflict        = errors.New("conflict")
)
