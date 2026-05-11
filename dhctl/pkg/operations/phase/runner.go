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

package phase

import (
	"context"
	"fmt"
)

// Runner executes a slice of Phase implementations in order. It carries no
// mutable state of its own and is safe to reuse across calls.
type Runner[State any] struct{}

// NewRunner returns a Runner parameterised for the given State type.
func NewRunner[State any]() *Runner[State] {
	return &Runner[State]{}
}

// Run executes phases in order. The first error stops the pipeline; the
// returned error is wrapped with the failing phase's Name so the operation
// log points at the failing step immediately.
func (r *Runner[State]) Run(ctx context.Context, state State, list []Phase[State]) error {
	for _, p := range list {
		if err := p.Run(ctx, state); err != nil {
			return fmt.Errorf("%s: %w", p.Name(), err)
		}
	}
	return nil
}
