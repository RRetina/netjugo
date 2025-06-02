package netjugo

import "errors"

var (
	ErrInvalidPrefix        = errors.New("invalid IP prefix")
	ErrInvalidMinPrefixLen  = errors.New("invalid minimum prefix length")
	ErrUnsupportedIPVersion = errors.New("unsupported IP version")
	ErrNilPointer           = errors.New("nil pointer reference")
	ErrFileNotFound         = errors.New("file not found")
	ErrInvalidFormat        = errors.New("invalid file format")
)
