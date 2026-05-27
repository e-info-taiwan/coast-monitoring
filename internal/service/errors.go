package service

import "errors"

var (
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("not found")
)
