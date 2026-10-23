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

package current

import (
	"context"
	"fmt"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
)

// startHooks runs once per enable; a failed start stops what it started (hooklane Start). The
// hooks live under Current's ctx, not the operation's: a binding that does not wait is
// synchronized after the sync that started it.
func (r *Reconciler) startHooks(ctx context.Context, st *operationState) error {
	if st.hooksStop != nil {
		return nil
	}

	stop, err := r.deps.Hooks.Start(ctx, r.ctx, r.pkg, r.changed)
	if err != nil {
		return fmt.Errorf("start hooks: %w", err)
	}

	st.hooksStop = stop

	return nil
}

// onStartup: the object skips OnStartup after a success until its hooks are disabled, so it runs
// once per enable.
func (r *Reconciler) onStartup(ctx context.Context) error {
	return r.runBinding(ctx, shtypes.OnStartup)
}

func (r *Reconciler) beforeHelm(ctx context.Context) error {
	return r.runBinding(ctx, addontypes.BeforeHelm)
}

// afterHelm records whether the hooks changed the values; reapply applies that inside this run.
func (r *Reconciler) afterHelm(ctx context.Context, st *operationState) error {
	before := r.pkg.GetValuesChecksum()

	if err := r.runBinding(ctx, addontypes.AfterHelm); err != nil {
		return err
	}

	st.reapply = before != r.pkg.GetValuesChecksum()

	return nil
}

// beforeDeleteHelm and afterDeleteHelm run only while the hooks run: a disable does not start
// them (disableSteps), an uninstall does (uninstallSteps).
func (r *Reconciler) beforeDeleteHelm(ctx context.Context, st *operationState) error {
	if st.hooksStop == nil {
		return nil
	}

	return r.runBinding(ctx, addontypes.BeforeDeleteHelm)
}

func (r *Reconciler) afterDeleteHelm(ctx context.Context, st *operationState) error {
	if st.hooksStop == nil {
		return nil
	}

	return r.runBinding(ctx, addontypes.AfterDeleteHelm)
}

// stopHooks stops the hook bindings and ends the enable: the next one runs OnStartup again.
func (r *Reconciler) stopHooks(st *operationState) error {
	if st.hooksStop != nil {
		st.hooksStop()
		st.hooksStop = nil
	}

	return nil
}

func (r *Reconciler) runBinding(ctx context.Context, binding shtypes.BindingType) error {
	if err := r.pkg.RunHooksByBinding(ctx, binding); err != nil {
		return fmt.Errorf("run %s hooks: %w", binding, err)
	}

	return nil
}
