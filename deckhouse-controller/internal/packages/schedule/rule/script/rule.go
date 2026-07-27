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

// Package script models a module's enabled script as a scheduler rule. A module
// may ship an `enabled` shell script that inspects the values of the
// already-enabled modules and reports whether this module should run. Its true
// result is a soft Enable vote and its false result a hard Forbid; a script that
// cannot be evaluated also forbids the module, so a broken script never leaves a
// module running by default.
package script

import (
	"context"

	addonutils "github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Reason constants attached to the decisions this rule emits. Each matches the
// Kubernetes condition reason pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$.
const (
	// reasonEnabledByScript is set when the enabled script ran and reported true.
	reasonEnabledByScript = "EnabledByScript"
	// reasonDisabledByScript is set when the enabled script ran and reported false.
	reasonDisabledByScript = "DisabledByScript"
	// reasonScriptError is set when the enabled script could not be run or parsed.
	reasonScriptError = "EnabledScriptError"
)

// Rule is driven by a module's enabled script: a true result votes to enable the
// module (a soft Enable), a false result vetoes it (Forbid), and a script that
// cannot be evaluated also vetoes it.
type Rule struct {
	pkg    Package
	logger *log.Logger
}

// Package is the slice of a module this rule inspects: its enabled script, if any.
type Package interface {
	// GetEnabledScriptDescriptor returns the module's enabled script descriptor, or nil when the
	// module ships none.
	GetEnabledScriptDescriptor() *Descriptor
}

// Descriptor describes a module's enabled script and the values handed to it. Values
// carries the global block (including the enabledModules list) the script reads;
// Settings carries the module's own config values.
type Descriptor struct {
	Path     string
	Settings addonutils.Values
	Values   addonutils.Values
}

// NewRule constructs a Rule that evaluates the given package's enabled script.
func NewRule(pkg Package, logger *log.Logger) *Rule {
	return &Rule{
		pkg:    pkg,
		logger: logger,
	}
}

// Decide runs the module's enabled script and folds its outcome onto the rule
// lattice: a missing script is no opinion (Undefined); a true result is a soft
// Enable vote; a false result vetoes the module (Forbid); a failure to run or
// parse the script also vetoes it, so a broken script cannot leave a module
// running by default. The rule.Rule interface carries no context and one must
// not be stored on the Rule, so the script runs under context.Background.
func (r *Rule) Decide() rule.Decision {
	script := r.pkg.GetEnabledScriptDescriptor()
	if script == nil {
		return rule.Decision{Kind: rule.Undefined}
	}

	res, err := runScript(context.Background(), script.Path, script.Settings, script.Values, r.logger)
	if err != nil {
		return rule.Decision{Kind: rule.Forbid, Reason: reasonScriptError, Message: err.Error()}
	}

	if !res.enabled {
		return rule.Decision{Kind: rule.Forbid, Reason: reasonDisabledByScript, Message: res.reason}
	}

	return rule.Decision{Kind: rule.Enable, Reason: reasonEnabledByScript}
}
