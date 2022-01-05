package warn

import "errors"

type warn struct {
	error
}

func (*warn) Warn() {}

// Warn returns a warning.
func Warn(msg string) error {
	return &warn{errors.New(msg)}
}
