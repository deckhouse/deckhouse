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

package lifecycle

import "context"

// RemovalState reports how far a package's teardown has got.
type RemovalState int

const (
	// RemovalDone means nothing is tracked under the name, so nothing is left to tear down.
	RemovalDone RemovalState = iota
	// RemovalPending means the package is still tracked and no teardown has been issued yet.
	RemovalPending
	// RemovalInFlight means a teardown is already enqueued; beginning a second one would cancel it.
	RemovalInFlight
)

// OperationKind is a lifecycle operation a package runs at most one of at a time.
type OperationKind uint8

const (
	// OperationUpdate deploys and loads a version: the pipeline a version change starts.
	OperationUpdate OperationKind = iota
	// OperationReconciliation configures, enables, runs or disables the loaded package.
	OperationReconciliation
	// OperationRemoval tears the package down; it outranks both others.
	OperationRemoval
)

// operation is a running lifecycle operation and the handle that cancels it.
type operation struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// BeginUpdate begins an update, cancelling the reconciliation and any update before it. It
// reports false for a package that is untracked or being removed, which owns no new work.
func (s *Store) BeginUpdate(name string) (context.Context, bool) {
	pkg, ok := s.packages[name]
	if !ok || pkg.removing {
		return nil, false
	}

	pkg.cancelOperation(
		OperationReconciliation,
		errUpdateStarted,
	)

	ctx := pkg.beginOperation(
		OperationUpdate,
		errUpdateSuperseded,
	)

	return ctx, true
}

// BeginReconciliation begins a reconciliation, superseding the one before it — this is what makes
// a schedule cancel an in-flight disable and back. A running update is left alone.
func (s *Store) BeginReconciliation(name string) (context.Context, bool) {
	pkg, ok := s.packages[name]
	if !ok || pkg.removing {
		return nil, false
	}

	ctx := pkg.beginOperation(
		OperationReconciliation,
		errReconciliationSuperseded,
	)

	return ctx, true
}

// BeginRemoval begins a teardown and cancels everything else the package runs. The state tells
// the caller whether there was anything to tear down; only RemovalPending carries a context.
func (s *Store) BeginRemoval(name string) (context.Context, RemovalState) {
	pkg, ok := s.packages[name]
	if !ok {
		return nil, RemovalDone
	}

	if pkg.removing {
		return nil, RemovalInFlight
	}

	pkg.removing = true

	pkg.cancelOperation(
		OperationUpdate,
		errRemovalStarted,
	)
	pkg.cancelOperation(
		OperationReconciliation,
		errRemovalStarted,
	)

	ctx := pkg.beginOperation(
		OperationRemoval,
		errRemovalStarted,
	)

	return ctx, RemovalPending
}

// CompleteRemoval drops the package once its teardown has finished, and reports whether it did.
func (s *Store) CompleteRemoval(name string) bool {
	pkg, ok := s.packages[name]
	if !ok || !pkg.removing {
		return false
	}

	op, ok := pkg.operation(OperationRemoval)
	if ok {
		op.cancel(nil)
	}

	delete(s.packages, name)

	return true
}
