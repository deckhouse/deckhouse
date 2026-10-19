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
// The admission side of the facade: what the validation webhooks ask the engine. It reads the
// scheduler, and the core's Reports and activated revisions.

package engine

import (
	"context"
	"errors"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

type exclusiveGrouped interface {
	GetExclusiveGroup() string
}

// CheckConstraints reports whether a node with constraints would be admitted now
// (schedule.Scheduler.CheckConstraints); name is the node name (apps.BuildName for an application).
func (e *Engine) CheckConstraints(name string, constraints schedule.Constraints) error {
	return e.scheduler.CheckConstraints(name, constraints)
}

// IsPackageEnabled reports the scheduler's latest decision about name; false before one arrived.
// Global is always enabled when the engine runs it.
func (e *Engine) IsPackageEnabled(name string) bool {
	if name == schedule.GlobalNode {
		return e.global != nil
	}

	report, ok := e.core.Report(name)

	return ok && !report.Removed && report.Current.Decision != nil && report.Current.Decision.Enabled
}

// GetEnabledModuleNames returns the modules the scheduler enabled, sorted; empty without
// EnableModuleV2.
func (e *Engine) GetEnabledModuleNames() []string {
	names := make([]string, 0)
	for _, name := range e.core.Names(model.KindModule) {
		if e.IsPackageEnabled(name) {
			names = append(names, name)
		}
	}

	return names
}

// ValidatePackageSettings converts and validates settings with the object of name's activated
// version, or with Global for global. Valid while the package has no activated object: the
// engine checks the settings itself before it activates one.
func (e *Engine) ValidatePackageSettings(ctx context.Context, name string, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error) {
	if name == schedule.GlobalNode {
		if e.global == nil {
			return settingscheck.Result{Valid: true}, nil
		}

		return e.global.ValidateSettings(ctx, settingsVersion, settings)
	}

	rev, ok := e.core.Activated(name)
	if !ok {
		return settingscheck.Result{Valid: true}, nil
	}

	return rev.Package.ValidateSettings(ctx, settingsVersion, settings)
}

// ValidateModuleExclusiveGroup returns an error when an enabled module belongs to group.
func (e *Engine) ValidateModuleExclusiveGroup(group string) error {
	for _, name := range e.GetEnabledModuleNames() {
		rev, ok := e.core.Activated(name)
		if !ok {
			continue
		}

		if grouped, ok := rev.Package.(exclusiveGrouped); ok && grouped.GetExclusiveGroup() == group {
			return errors.New("module cannot be enabled because another module with same exclusiveGroup enabled")
		}
	}

	return nil
}
