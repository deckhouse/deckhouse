/*
Copyright 2022 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deckhouse_config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/deckhouse-config/conversion"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type Status struct {
	State   string
	Version string
	Status  string
	Type    string
}

type StatusReporter struct {
	moduleManager ModuleManager
	possibleNames set.Set
}

func NewModuleInfo(mm ModuleManager, possibleNames set.Set) *StatusReporter {
	return &StatusReporter{
		moduleManager: mm,
		possibleNames: possibleNames,
	}
}

func (s *StatusReporter) ForModule(module *v1alpha1.Module, bundleName string, modulesToSource map[string]string, enabled bool) Status {
}

func (s *StatusReporter) ForConfig(cfg *v1alpha1.ModuleConfig, bundleName string, modulesToSource map[string]string, enabled bool) Status {
	// Special case: unknown module name.
	moduleType, fromSource := modulesToSource[cfg.GetName()]

	if !s.possibleNames.Has(cfg.GetName()) && !fromSource {
		return Status{
			State:   "Ignored: unknown module name",
			Version: "",
		}
	}

	chain := conversion.Registry().Chain(cfg.GetName())

	// Run conversions and validate versioned settings to warn about invalid spec.settings.
	// TODO(future): add cache for these errors, for example in internal values.
	if chain.IsKnownVersion(cfg.Spec.Version) && hasVersionedSettings(cfg) {
		res := Service().ConfigValidator().Validate(cfg)
		if res.HasError() {
			var prefix = "Ignored"
			if enabled {
				prefix = "Error"
			}

			invalidMsg := fmt.Sprintf("%s: %s", prefix, res.Error)
			return Status{
				State:   invalidMsg,
				Version: "",
			}
		}
	}

	// Fill the 'version' field. The value is a spec.version or the latest version from registered conversions.
	// Also create warning if version is unknown or outdated.
	versionWarning := ""
	version := ""
	if cfg.Spec.Version == 0 {
		// Use latest version if spec.version is empty.
		version = strconv.Itoa(chain.LatestVersion())
	}
	if cfg.Spec.Version > 0 {
		version = strconv.Itoa(cfg.Spec.Version)
		if !chain.IsKnownVersion(cfg.Spec.Version) {
			versionWarning = fmt.Sprintf("Error: invalid spec.version, use version %d", chain.LatestVersion())
		} else if chain.Conversion(cfg.Spec.Version) != nil {
			// Warn about obsolete version if there is conversion for spec.version.
			versionWarning = fmt.Sprintf("Update available, latest spec.settings schema version is %d", chain.LatestVersion())
		}
	}

	// 'global' config is always enabled.
	if cfg.GetName() == "global" {
		return Status{
			State:   fmt.Sprintf("%s: %s", "Enabled", versionWarning),
			Version: version,
		}
	}

	// Figure out additional statuses for known modules.
	statusMsgs := make([]string, 0)
	if versionWarning != "" {
		statusMsgs = append(statusMsgs, versionWarning)
	}

	mod := s.moduleManager.GetModule(cfg.GetName())

	// Calculate state and status.
	stateMsg := "Disabled"
	if s.moduleManager.IsModuleEnabled(cfg.GetName()) {
		stateMsg = "Enabled"

		lastHookErr := mod.GetLastHookError()
		if lastHookErr != nil {
			statusMsgs = append(statusMsgs, fmt.Sprintf("HookError: %v", lastHookErr))
		}
		if mod.GetModuleError() != nil {
			statusMsgs = append(statusMsgs, fmt.Sprintf("ModuleError: %v", mod.GetModuleError()))
		}

		if len(statusMsgs) == 0 { // no errors were added
			// Best effort alarm!
			//
			// Actually, this condition is not correct because the `CanRunHelm` status appears right before the first run.c
			// The right approach is to check the queue for the module run task.
			// However, there are too many addon-operator internals involved.
			// We should consider moving these statuses to the `Module` resource,
			// which is directly controlled by addon-operator.
			if mod.GetPhase() == modules.CanRunHelm {
				statusMsgs = append(statusMsgs, "Ready")
			} else {
				statusMsgs = append(statusMsgs, "Converging: module is waiting for the first run")
			}
		}
	} else {
		// Special case: no enabled flag in ModuleConfig, module disabled by bundle.
		if cfg.Spec.Enabled == nil {
			// Consider merged static enabled flags as '*Enabled flags from the bundle'.
			enabledMsg := "disabled"
			// TODO(yalosev): think about it
			if s.moduleManager.IsModuleEnabled(mod.GetName()) {
				enabledMsg = "enabled"
			}
			statusMsgs = append(statusMsgs, fmt.Sprintf("Info: %s by %s bundle", enabledMsg, bundleName))
		}

		// Special case: explicitly enabled by the config but effectively disabled by the ModuleManager.
		if cfg.Spec.Enabled != nil && *cfg.Spec.Enabled {
			statusMsgs = append(statusMsgs, "Info: turned off by 'enabled'-script, refer to the module documentation")
		}
	}

	return Status{
		Version: version,
		State:   fmt.Sprintf("%s: %s", stateMsg, strings.Join(statusMsgs, ", "),
	}
}
