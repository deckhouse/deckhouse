/*
Copyright 2026 Flant JSC

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

// Package requirements gates a Deckhouse update on which registry implementation the cluster runs.
//
// It ships one release ahead of the release it protects, because a requirement is evaluated by the
// version already installed. Without it, the release that removes the legacy implementation would
// land on a cluster still running it and take away the code that configures the container runtime on
// that cluster's nodes — in a release that itself has to be pulled through that configuration.
package requirements

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	// ImplementationKey is what the module records and a release may require.
	//
	// May, not does: no release declares it yet, and it cannot be this one — a requirement is
	// evaluated by the version already INSTALLED, and a cluster on an older version has never
	// registered this check and lets any value through. This release registers it and records a
	// value everywhere; the release that refuses to install on a cluster still running the previous
	// implementation adds `"registryImplementation": "V2"` to release.yaml.
	ImplementationKey = "registryImplementation"
)

func init() {
	requirements.RegisterCheck(ImplementationKey, check)
}

// check compares what a release requires against what the cluster is running.
//
// A value that is absent or not a string refuses. The module's hook seeds Legacy when the process
// starts and replaces it on its first pass, so neither happens while the module is enabled; if a
// change ever breaks that, a release that removes code this cluster may still depend on is the
// wrong thing to let through on missing information. With the module disabled this check is not
// consulted at all.
func check(required string, getter requirements.ValueGetter) (bool, error) {
	raw, exists := getter.Get(ImplementationKey)
	current, ok := raw.(string)
	if !exists || !ok {
		return false, fmt.Errorf(
			"this release requires the %q registry implementation, and the `registry` module has "+
				"not recorded which one this cluster depends on; wait for the module to reconcile",
			required)
	}
	if current == required {
		return true, nil
	}

	// The advice names what an operator can do on THIS release, mode by mode, because no setting
	// selects an implementation: the handover happens on its own after the update, once the previous
	// implementation has let go of the pull path or, in `Direct`, once its replacement is configured.
	return false, fmt.Errorf(
		"this release requires the %q registry implementation and removes the previous (v1) "+
			"implementation of the `registry` module, which this cluster still depends on (%q); "+
			"before the update, migrate as described in \"Migrating from the previous implementation "+
			"to the current one\" on the documentation page \"Managing the registry in DP-managed "+
			"clusters\": from `Proxy`, set `registry.mode` in the `deckhouse` ModuleConfig to "+
			"`Unmanaged` and wait for the switch to finish; from `Local`, follow the procedure for "+
			"air-gapped clusters; from `Direct`, configure the `registry` ModuleConfig with "+
			"`mode: Managed` and `primary.upstream`; this requirement is met on its own once that is done",
		required, current)
}
