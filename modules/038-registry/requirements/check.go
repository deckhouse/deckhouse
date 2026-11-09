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
	// A requirement is evaluated by the version already INSTALLED, and a cluster on a version that
	// never registered this check lets any value through. It was registered in 1.77.1, and 1.78 —
	// the release without the v1 implementation — requires `"registryImplementation": "V2"` in
	// release.yaml. Here it keeps the following releases off a cluster that 1.78 refused to take
	// over from.
	ImplementationKey = "registryImplementation"
)

func init() {
	requirements.RegisterCheck(ImplementationKey, check)
}

// check compares what a release requires against what the cluster is running.
//
// An absent value passes: a cluster that recorded nothing is one whose registry module has not run
// yet, and blocking an update on missing information would strand it.
func check(required string, getter requirements.ValueGetter) (bool, error) {
	raw, exists := getter.Get(ImplementationKey)
	if !exists {
		return true, nil
	}

	current, ok := raw.(string)
	if !ok {
		return true, nil
	}
	if current == required {
		return true, nil
	}

	// On this release the v1 implementation is gone, and with it the lever the release before
	// named here: `registry.mode` of the deckhouse ModuleConfig moves nothing any more. No setting
	// selects an implementation either. So the message points at the gate's own reason, which names
	// the one step left where there is one — configuring the module, for `Direct` — and says that
	// any other state is not supported.
	return false, fmt.Errorf(
		"this release requires the %q registry implementation, and the cluster is running %q: "+
			"the registry module has not taken over from the v1 implementation; the reason is in "+
			"the D8RegistryMigrationPending alert. For the Direct mode, set `mode: Managed` with "+
			"`primary.upstream` in the registry ModuleConfig; any other reason is a state that is "+
			"not supported, contact Deckhouse Platform technical support",
		required, current)
}
