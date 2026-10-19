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

// Package operation runs one operation of Current, Global or the package Reconciler (the
// preparation of a candidate): its steps, in order, in a worker goroutine that the owner's loop
// starts and cancels. It decides nothing and reports nothing; the steps are closures over a
// per-operation state the owner keeps, so this package knows nothing of its owners.
package operation

import "context"

// Kind names an operation.
type Kind string

const (
	Run       Kind = "run"
	Disable   Kind = "disable"
	Uninstall Kind = "uninstall"
	Sync      Kind = "sync"
	Prepare   Kind = "prepare" // Prepare attaches and loads a candidate (the package Reconciler's).
)

// Condition is the condition a failing step reports on; unused by Prepare, whose failure goes to
// the Candidate section.
type Condition int

const (
	HooksProcessed Condition = iota
	ManifestsApplied
)

// Step is one named unit of an operation; Condition and Reason say what its failure reports. Run
// executes in the worker goroutine.
type Step struct {
	Name      string
	Condition Condition
	Reason    string
	Run       func(ctx context.Context) error
}

// Result is what an operation ended with.
type Result struct {
	Kind Kind
	Key  string
	// Failed is the step that failed; nil on success.
	Failed *Step
	Err    error
	// Canceled: the operation's context was cancelled when the step failed. The owner cancelled
	// it, so it is not a failure.
	Canceled bool
}

// Operation is one operation in progress.
type Operation struct {
	Kind Kind
	// Key identifies what the operation started with (the Inputs of a run or a disable), so the
	// owner can tell whether a newer decision supersedes it.
	Key string

	cancel context.CancelFunc
	done   chan Result // done has capacity 1 and receives exactly one Result.
}

// Start runs steps in order in a new goroutine under a child of ctx and stops at the first
// failure. Non-blocking.
func Start(ctx context.Context, kind Kind, key string, steps []Step) *Operation {
	ctx, cancel := context.WithCancel(ctx)

	o := &Operation{Kind: kind, Key: key, cancel: cancel, done: make(chan Result, 1)}

	go func() {
		defer cancel()

		o.done <- run(ctx, kind, key, steps)
	}()

	return o
}

// Cancel cancels the operation's context; the result still arrives on Done. Idempotent.
func (o *Operation) Cancel() {
	o.cancel()
}

// Done delivers the result once, when the last step returned. Nil-safe: a nil Operation's channel
// is nil and blocks, so an idle owner selects on it unchanged.
func (o *Operation) Done() <-chan Result {
	if o == nil {
		return nil
	}

	return o.done
}

// run executes steps and builds the result.
func run(ctx context.Context, kind Kind, key string, steps []Step) Result {
	for i := range steps {
		if err := steps[i].Run(ctx); err != nil {
			return Result{Kind: kind, Key: key, Failed: &steps[i], Err: err, Canceled: ctx.Err() != nil}
		}
	}

	return Result{Kind: kind, Key: key}
}
