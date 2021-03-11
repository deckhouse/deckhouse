package check

import (
	"fmt"
)

func ErrFail(format string, a ...interface{}) Error {
	return checkError{
		err:    fmt.Errorf(format, a...),
		result: StatusFail,
	}
}

func ErrUnknown(format string, a ...interface{}) Error {
	return checkError{
		err:    fmt.Errorf(format, a...),
		result: StatusUnknown,
	}
}

type Error interface {
	Error() string
	Status() Status
}

type checkError struct {
	err    error
	result Status
}

func (pe checkError) Error() string {
	return pe.err.Error()
}

func (pe checkError) Status() Status {
	return pe.result
}
