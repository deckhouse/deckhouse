/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
