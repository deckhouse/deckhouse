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

	// The advice names the only lever there is: no setting selects an implementation, and the
	// handover happens on its own once the previous implementation has let go of the pull path.
	return false, fmt.Errorf(
		"this release requires the %q registry implementation, and the cluster is running %q; "+
			"bring `registry.mode` in the deckhouse ModuleConfig to Unmanaged and wait for that "+
			"transition to finish, after which the handover completes on its own",
		required, current)
}
