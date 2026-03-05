package resolver

import "errors"

var (
	ErrNotFound           = errors.New("file not found")
	ErrTemplateNotFound   = errors.New("template not found")
	ErrMaxDepth           = errors.New("max template recursion depth exceeded")
	ErrInvalidTemplateRef = errors.New("invalid template reference")
	ErrNoLoader           = errors.New("no resource loader configured")
	ErrInvalidYAML        = errors.New("invalid YAML")
)
