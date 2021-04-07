package check

import (
	"fmt"
)

func ErrFail(format string, a ...interface{}) Error {
	return checkError{
		err:    fmt.Errorf(format, a...),
		status: Down,
	}
}

func ErrUnknown(format string, a ...interface{}) Error {
	return checkError{
		err:    fmt.Errorf(format, a...),
		status: Unknown,
	}
}

type Error interface {
	Error() string
	Status() Status
}

type checkError struct {
	err    error
	status Status
}

func (e checkError) Error() string {
	return e.err.Error()
}

func (e checkError) Status() Status {
	return e.status
}
