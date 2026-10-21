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

// Package script is a module's enabled script as a scheduler rule. The script is veto-only: true
// has no opinion and cannot turn a module on; false is a Forbid. A script that cannot be evaluated
// keeps its last verdict, so a transient failure does not flip a running module; with no verdict
// yet it is a Forbid.
package script

import (
	"context"
	"log/slog"
	"maps"
	"strings"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	reasonEnabledByScript  = "EnabledByScript"
	reasonDisabledByScript = "DisabledByScript"
	reasonScriptError      = "EnabledScriptError" // reasonScriptError: the script could not be run or parsed.
)

// Rule is the enabled-script rule of one module.
type Rule struct {
	descriptor     func() *Descriptor
	enabledModules func() []string
	// inputs identifies what the script reads besides the enabled modules: equal inputs and enabled
	// modules give the last verdict without a run.
	inputs func() string
	// last is the verdict of the last successful run, lastKey its inputs; nil before the first one.
	last    *rule.Decision
	lastKey string
	logger  *log.Logger
}

// Descriptor is a module's enabled script with its inputs: Values with the global block, Settings
// the module's own settings. The rule replaces global.enabledModules in Values before the run.
type Descriptor struct {
	Path     string
	Settings addonutils.Values
	Values   addonutils.Values
}

// NewRule constructs the rule of the script descriptor returns (nil: none). enabledModules is what
// the script sees as global.enabledModules; inputs identifies the settings and values the script
// reads (nil: run on every decision); logger should name the module.
func NewRule(descriptor func() *Descriptor, enabledModules func() []string, inputs func() string, logger *log.Logger) *Rule {
	return &Rule{
		descriptor:     descriptor,
		enabledModules: enabledModules,
		inputs:         inputs,
		logger:         logger,
	}
}

// Decide runs the script, under context.Background: rule.Rule carries no context. It has no
// timeout, accepted: the script runs under the scheduler's mutex, so a hung script would block
// every scheduler call; the scripts are small and ship with deckhouse. The error is only logged:
// the status keeps showing the last verdict.
func (r *Rule) Decide() rule.Decision {
	enabled := r.enabledModules()

	// A run is an exec plus the values marshaled to files: on equal inputs the verdict is the same.
	key := ""
	if r.inputs != nil {
		key = r.inputs() + "\nenabled=" + strings.Join(enabled, ",")
		if r.last != nil && key == r.lastKey {
			return *r.last
		}
	}

	script := r.descriptor()
	if script == nil {
		return rule.Decision{Kind: rule.Undefined}
	}

	values := withEnabledModules(script.Values, enabled)

	res, err := runScript(context.Background(), script.Path, script.Settings, values, r.logger)
	if err != nil {
		if r.last != nil {
			r.logger.Warn("enabled script failed, keep the last verdict", slog.String("path", script.Path), log.Err(err))
			return *r.last
		}

		return rule.Decision{Kind: rule.Forbid, Reason: reasonScriptError, Message: err.Error()}
	}

	decision := rule.Decision{Kind: rule.Undefined, Reason: reasonEnabledByScript}
	if !res.enabled {
		decision = rule.Decision{Kind: rule.Forbid, Reason: reasonDisabledByScript, Message: res.reason}
	}

	r.last, r.lastKey = &decision, key

	return decision
}

// withEnabledModules returns values with global.enabledModules set to modules, without changing
// values or its global block: both may be live maps of the objects.
func withEnabledModules(values addonutils.Values, modules []string) addonutils.Values {
	out := make(addonutils.Values, len(values)+1)
	maps.Copy(out, values)

	global := make(addonutils.Values)

	switch g := values["global"].(type) {
	case addonutils.Values:
		maps.Copy(global, g)
	case map[string]any:
		maps.Copy(global, g)
	}

	global["enabledModules"] = modules
	out["global"] = global

	return out
}
