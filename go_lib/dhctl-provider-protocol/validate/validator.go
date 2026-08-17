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

// Package validate is the validate action: its payload, its result, the function a
// plugin implements, the rules that hold regardless of how the call arrived, and the
// translation to and from the wire types in gen/validate/v1.
//
// A plugin depends on this package and on nothing else of the protocol; the server
// and client packages are the two halves of the transport around it.
package validate

import (
	"context"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

type Validator interface {
	Validate(ctx context.Context, input Input) (Result, error)
}

type validator struct {
	fn ValidateFn
}

type ValidateFn func(ctx context.Context, input Input) (Result, error)

func NewValidator(fn ValidateFn) Validator {
	return &validator{
		fn: fn,
	}
}

func (i *validator) Validate(ctx context.Context, input Input) (Result, error) {
	if i.fn == nil {
		return Result{}, errs.ErrMethodUnimplemented
	}

	if err := input.Validate(); err != nil {
		return Result{}, err
	}

	return i.fn(ctx, input)
}
