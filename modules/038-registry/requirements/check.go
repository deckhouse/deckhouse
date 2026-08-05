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

// Package requirements gates a Deckhouse update on which registry implementation the
// cluster is running.
//
// The reason it exists now, before anything needs it: the release that finally removes
// the legacy implementation must not be installable on a cluster still running it. That
// cluster would lose the code that configures the container runtime on its nodes, and it
// would lose it in the release that also has to be pulled through that very
// configuration. The check has to be present in the releases *before* that one, because
// a requirement is evaluated by the version already installed.
package requirements

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	// ImplementationKey is what the module records and a release requires.
	ImplementationKey = "registryImplementation"
)

func init() {
	requirements.RegisterCheck(ImplementationKey, check)
}

// check compares what a release requires against what the cluster is running.
//
// An absent value passes. A cluster that has never recorded one is a cluster whose
// registry module has not run yet, and blocking an update on the absence of information
// would strand it.
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

	return false, fmt.Errorf(
		"this release requires the %q registry implementation, and the cluster is running %q; "+
			"set 'implementation: %s' in the registry ModuleConfig and wait for the switch to complete",
		required, current, required)
}
