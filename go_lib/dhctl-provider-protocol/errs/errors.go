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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusInvalidRequest(err error) error {
	return status.Error(codes.InvalidArgument, err.Error())
}

func StatusInternal(err error) error {
	return status.Error(codes.Internal, err.Error())
}

// IsStatusErr reports whether err already carries a gRPC status — either directly or
// wrapped, since status.FromError unwraps. A validator that built its error with
// status picked the code deliberately, and the server must not override it.
func IsStatusErr(err error) bool {
	_, ok := status.FromError(err)
	return ok
}
