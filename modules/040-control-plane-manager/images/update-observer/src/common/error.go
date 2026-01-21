package common

import (
	"errors"
	"fmt"
)

type ReconcileTolerantError struct {
	Err     error
	Message string
}

func (e *ReconcileTolerantError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("ReconcileTolerantError: %s", e.Message)
	}
	return fmt.Sprintf("ReconcileTolerantError: %v", e.Err)
}

func (e *ReconcileTolerantError) Unwrap() error {
	return e.Err
}

func NewReconcileTolerantError(msg string, args ...any) error {
	return &ReconcileTolerantError{
		Message: fmt.Sprintf(msg, args...),
	}
}

func WrapIntoReconcileTolerantError(err error, msg string, args ...any) error {
	return &ReconcileTolerantError{
		Err:     err,
		Message: fmt.Sprintf(msg, args...),
	}
}

func IsReconcileTolerantError(err error) bool {
	var tolerantErr *ReconcileTolerantError
	return errors.As(err, &tolerantErr)
}
