package checks

import (
	"fmt"
)

func ErrFail(format string, a ...interface{}) Error {
	return probeError{
		err:    fmt.Errorf(format, a...),
		result: StatusFail,
	}
}

func ErrUnknownResult(format string, a ...interface{}) Error {
	return probeError{
		err:    fmt.Errorf(format, a...),
		result: StatusUnknown,
	}
}

type Error interface {
	Error() string
	Status() Status
}

type probeError struct {
	err    error
	result Status
}

func (pe probeError) Error() string {
	return pe.err.Error()
}

func (pe probeError) Status() Status {
	return pe.result
}
