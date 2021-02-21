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

func ErrNoData(format string, a ...interface{}) Error {
	return probeError{
		err:    fmt.Errorf(format, a...),
		result: StatusNoData,
	}
}

type Error interface {
	Error() string
	Result() Status
}

type probeError struct {
	err    error
	result Status
}

func (pe probeError) Error() string {
	return pe.err.Error()
}

func (pe probeError) Result() Status {
	return pe.result
}
