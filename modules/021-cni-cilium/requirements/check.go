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

func checkCNIConfigurationSettledFunc(_ string, getter requirements.ValueGetter) (bool, error) {
	cniConfigurationSettledStatusRaw, exists := getter.Get("cniConfigurationSettled")
	if !exists {
		return true, nil
	}

	if cniConfigurationSettledStatus, ok := cniConfigurationSettledStatusRaw.(string); ok {
		if cniConfigurationSettledStatus == "false" {
			return false, errors.New(
				"A problem has been found in the CNI configuration, see ClusterAlerts for details",
			)
		}
	}
	return true, nil
}

func checkMinimalKernelVersionFunc(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	// Minimal version of the Linux kernel on the node in the cluster
	currentMinimalLinuxKernelVersion, exists := getter.Get("currentMinimalLinuxKernelVersion")
	if !exists {
		fmt.Println("[DEBUG] Key 'currentMinimalLinuxKernelVersion' does not exists in requirements")
		return true, nil
	}
	currentMinimalLinuxKernelVersionSemVer, err := semver.NewVersion(strings.Split(currentMinimalLinuxKernelVersion.(string), "-")[0])
	if err != nil {
		return false, fmt.Errorf("unable to parse current minimal Linux kernel version: %w", err)
	}

	// Required minimal Linux kernel version
	if requirementValue == "" {
		fmt.Println("[DEBUG] Key 'nodesMinimalLinuxKernelVersion' does not exists in release.yaml")
		return true, nil
	}
	nodesMinimalLinuxKernelVersionSemVer, err := semver.NewVersion(strings.Split(requirementValue, "-")[0])
	if err != nil {
		return false, fmt.Errorf("unable to parse current minimal Linux kernel version: %w", err)
	}

	// Compare versions
	if currentMinimalLinuxKernelVersionSemVer.LessThan(nodesMinimalLinuxKernelVersionSemVer) {
		return false, fmt.Errorf(
			"the current version of the Linux kernel on the cluster nodes is less than %s, which is required by the module",
			requirementValue,
		)
	}

	return true, nil
}
