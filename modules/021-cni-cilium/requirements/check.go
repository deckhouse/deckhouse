/*
Copyright 2024 Flant JSC

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

package requirements

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

func init() {
	requirements.RegisterCheck("nodesMinimalLinuxKernelVersion", checkMinimalKernelVersionFunc)
	requirements.RegisterCheck("cniConfigurationSettled", checkCNIConfigurationSettledFunc)
}

// checks whether the CNI configuration was successfully settled.
// If it's "false", the module will not be enabled.
func checkCNIConfigurationSettledFunc(_ string, getter requirements.ValueGetter) (bool, error) {
	rawValue, found := getter.Get("cniConfigurationSettled")
	if !found {
		return true, nil
	}

	if status, ok := rawValue.(string); ok && status == "false" {
		return false, errors.New(
			"a problem has been found in the CNI configuration; see ClusterAlerts for details",
		)
	}

	return true, nil
}

// checks that the current minimal kernel version across cluster nodes
// satisfies the required constraint declared in release.yaml.
func checkMinimalKernelVersionFunc(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	rawCurrentVersion, found := getter.Get("currentMinimalLinuxKernelVersion")
	if !found {
		// Key not available; assume requirement passes.
		return true, nil
	}

	currentVersionStr, ok := rawCurrentVersion.(string)
	if !ok {
		return false, fmt.Errorf("invalid type for current minimal kernel version: %T", rawCurrentVersion)
	}

	currentSemver, err := parseKernelSemver(currentVersionStr)
	if err != nil {
		return false, fmt.Errorf("unable to parse current minimal Linux kernel version: %w", err)
	}

	if requirementValue == "" {
		// If the requirement is not set, pass by default.
		return true, nil
	}

	requiredSemver, err := parseKernelSemver(requirementValue)
	if err != nil {
		return false, fmt.Errorf("unable to parse required minimal Linux kernel version: %w", err)
	}

	if currentSemver.LessThan(requiredSemver) {
		return false, fmt.Errorf(
			"the current Linux kernel version on cluster nodes (%s) is lower than required (%s)",
			currentSemver, requiredSemver,
		)
	}

	return true, nil
}

func parseKernelSemver(version string) (*semver.Version, error) {
	base := strings.SplitN(version, "-", 2)[0]
	return semver.NewVersion(base)
}
