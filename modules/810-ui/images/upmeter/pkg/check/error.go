/*
Copyright 2023 Flant JSC

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
