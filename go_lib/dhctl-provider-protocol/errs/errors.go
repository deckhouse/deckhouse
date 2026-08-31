// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errs

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrInvalidRequest      = errors.New("invalid request")
	ErrMethodUnimplemented = errors.New("method unimplemented")
)

func MapToStatus(err error) error {
	switch {
	case err == nil:
		return nil

	case errors.Is(err, ErrInvalidRequest):
		return StatusInvalidRequest(err)

	case errors.Is(err, ErrMethodUnimplemented):
		return StatusMethodUnimplemented(err)

	default:
		return StatusInternal(err)
	}
}

func StatusInvalidRequest(err error) error {
	return status.Error(codes.InvalidArgument, err.Error())
}

func StatusMethodUnimplemented(err error) error {
	return status.Error(codes.Unimplemented, err.Error())
}

func StatusInternal(err error) error {
	return status.Error(codes.Internal, err.Error())
}
