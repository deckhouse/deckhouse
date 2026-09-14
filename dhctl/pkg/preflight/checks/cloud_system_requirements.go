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

package checks

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type CloudSystemRequirementsCheck struct {
	InstallConfig *config.DeckhouseInstaller
}

const CloudSystemRequirementsCheckName preflight.CheckName = "cloud-master-system-requirements"

func (CloudSystemRequirementsCheck) Description() string {
	return "cloud master node system requirements are met"
}

func (CloudSystemRequirementsCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (CloudSystemRequirementsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c CloudSystemRequirementsCheck) Run(_ context.Context) (string, error) {
	// In the ModuleConfig-only flow (no <Provider>ClusterConfiguration in the
	// input file) the master sizing lives in NodeGroup + InstanceClass resources
	// resolved by the provider validator, not in PCC.
	if c.InstallConfig == nil || len(c.InstallConfig.ProviderClusterConfig) == 0 {
		return "", preflight.NotApplicable("there is no <Provider>ClusterConfiguration to read the master sizing from")
	}

	requirements := systemRequirementsForConfig(c.InstallConfig)

	configObject := make(map[string]any)
	configKind, err := unmarshalProviderClusterConfiguration(c.InstallConfig.ProviderClusterConfig, configObject)
	if err != nil {
		return "", fmt.Errorf("unmarshal provider cluster configuration: %w", err)
	}

	paths, known := masterSizingPaths[configKind]
	if !known {
		// External providers ship their own validator binary that checks master sizing from
		// NodeGroup/InstanceClass.
		return "", preflight.NotApplicable("%s is checked by the provider's own validator", configKind)
	}

	// Every violation, not the first: the three values are independent, and reporting them one
	// per run meant a master that is short on CPU *and* RAM cost two bootstrap attempts to find
	// out about.
	var violations []string
	var reported []string

	disk, err := readIntegerProperty(configObject, paths.rootDisk)
	switch {
	case err != nil:
		return "", c.malformed(configKind, paths.rootDisk, err)
	case disk == nil:
		// allowMissing: several providers take the root disk size from the image or the flavor.
	case *disk < requirements.rootDiskSizeGB:
		violations = append(violations, fmt.Sprintf("%s: %d GB configured, at least %d GB required",
			pathString(paths.rootDisk), *disk, requirements.rootDiskSizeGB))
	default:
		reported = append(reported, fmt.Sprintf("%d GB root disk", *disk))
	}

	memory, err := readIntegerProperty(configObject, paths.memory)
	switch {
	case err != nil:
		return "", c.malformed(configKind, paths.memory, err)
	case memory == nil && len(paths.memory) > 0:
		return "", c.missing(configKind, paths.memory)
	case memory != nil && *memory < requirements.memoryMB:
		violations = append(violations, fmt.Sprintf("%s: %d MB configured, at least %d MB required (%d GB minus %d MB tolerance)",
			pathString(paths.memory), *memory, requirements.memoryMB, (requirements.memoryMB+reservedMemoryThresholdMB)/1024, reservedMemoryThresholdMB))
	case memory != nil:
		reported = append(reported, fmt.Sprintf("%d MB RAM", *memory))
	}

	cores, err := readIntegerProperty(configObject, paths.cores)
	switch {
	case err != nil:
		return "", c.malformed(configKind, paths.cores, err)
	case cores == nil && len(paths.cores) > 0:
		return "", c.missing(configKind, paths.cores)
	case cores != nil && *cores < requirements.cpuCores:
		violations = append(violations, fmt.Sprintf("%s: %d configured, at least %d required",
			pathString(paths.cores), *cores, requirements.cpuCores))
	case cores != nil:
		reported = append(reported, fmt.Sprintf("%d CPU", *cores))
	}

	if len(violations) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("%s.masterNodeGroup.instanceClass", configKind),
			Observed: "- " + strings.Join(violations, "\n- "),
			Expected: fmt.Sprintf("at least %d CPU, %d MB RAM and %d GB of root disk",
				requirements.cpuCores, requirements.memoryMB, requirements.rootDiskSizeGB),
			Fix: fmt.Sprintf("raise these values in %s, or set bundle: Minimal in the \"deckhouse\" ModuleConfig (%d CPU / %d MB)",
				configKind, minimalBundleRequiredCPUCores, minimalBundleRequiredMemoryMB),
		})
	}

	if len(paths.cores) == 0 && len(paths.memory) == 0 {
		// The flavour providers: the CPU and RAM come from an instance type dhctl cannot size.
		return fmt.Sprintf("master %s checked; CPU and RAM come from the instance type and are not verified",
			strings.Join(reported, ", ")), nil
	}
	return fmt.Sprintf("master instanceClass has %s", strings.Join(reported, ", ")), nil
}

// masterSizingPaths is where each provider keeps the three numbers. An empty path means the
// provider does not carry that value in its cluster configuration: AWS, GCP, Azure, OpenStack,
// VCD and Huawei size CPU and RAM through an instance type or flavour, which dhctl cannot resolve
// without calling the cloud API.
var masterSizingPaths = map[string]struct{ cores, memory, rootDisk []string }{
	"AWSClusterConfiguration":         {rootDisk: []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}},
	"GCPClusterConfiguration":         {rootDisk: []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}},
	"AzureClusterConfiguration":       {rootDisk: []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}},
	"OpenStackClusterConfiguration":   {rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSize"}},
	"VCDClusterConfiguration":         {rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"}},
	"HuaweiCloudClusterConfiguration": {rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSize"}},

	"YandexClusterConfiguration": {
		cores:    []string{"masterNodeGroup", "instanceClass", "cores"},
		memory:   []string{"masterNodeGroup", "instanceClass", "memory"},
		rootDisk: []string{"masterNodeGroup", "instanceClass", "diskSizeGB"},
	},
	"VsphereClusterConfiguration": {
		cores:    []string{"masterNodeGroup", "instanceClass", "numCPUs"},
		memory:   []string{"masterNodeGroup", "instanceClass", "memory"},
		rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSize"},
	},
	"ZvirtClusterConfiguration": {
		cores:    []string{"masterNodeGroup", "instanceClass", "numCPUs"},
		memory:   []string{"masterNodeGroup", "instanceClass", "memory"},
		rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"},
	},
	"DynamixClusterConfiguration": {
		cores:    []string{"masterNodeGroup", "instanceClass", "numCPUs"},
		memory:   []string{"masterNodeGroup", "instanceClass", "memory"},
		rootDisk: []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"},
	},
}

func (c CloudSystemRequirementsCheck) malformed(configKind string, path []string, err error) error {
	return preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("%s.%s", configKind, pathString(path)),
		Observed: err.Error(),
		Expected: "an integer",
		Fix:      fmt.Sprintf("write %s as a plain number, without quotes or a unit suffix", pathString(path)),
	})
}

func (c CloudSystemRequirementsCheck) missing(configKind string, path []string) error {
	return preflight.Permanent(&preflight.Failure{
		Checked:  fmt.Sprintf("%s.%s", configKind, pathString(path)),
		Observed: "the field is not set",
		Expected: "the size of the master node",
		Fix:      fmt.Sprintf("set %s in the %s document", pathString(path), configKind),
	})
}

func pathString(path []string) string {
	return strings.Join(path, ".")
}

func CloudSystemRequirements(installConfig *config.DeckhouseInstaller) preflight.Check {
	check := CloudSystemRequirementsCheck{InstallConfig: installConfig}
	return preflight.Check{
		Name:        CloudSystemRequirementsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Cacheable:   true,
		Run:         check.Run,
	}
}

// readIntegerProperty reads one number out of the parsed configuration. It returns nil when the
// field is absent, which is a legitimate answer for the providers that do not carry it.
//
// The type is decided at run time rather than asserted: the previous code wrote propertyValue.(int)
// and panicked on `memory: "8192"` and on `memory: 8192.0`, both of which YAML accepts and
// operators write. A panic in a preflight check is the worst possible outcome — it takes down the
// run that was being checked, with a Go stack trace in place of the sentence saying what is wrong.
func readIntegerProperty(configObject map[string]any, propertyPath []string) (*int, error) {
	if len(propertyPath) == 0 {
		return nil, nil
	}
	propertyValue, found, err := unstructured.NestedFieldNoCopy(configObject, propertyPath...)
	if err != nil {
		return nil, fmt.Errorf("malformed provider cluster configuration: %w", err)
	}
	if !found {
		return nil, nil
	}

	switch value := propertyValue.(type) {
	case int:
		return &value, nil
	case int64:
		converted := int(value)
		return &converted, nil
	case float64:
		// YAML gives a float for 8192.0; only a whole number is meaningful here.
		if value != float64(int(value)) {
			return nil, fmt.Errorf("%v is not a whole number", value)
		}
		converted := int(value)
		return &converted, nil
	case string:
		return nil, fmt.Errorf("%q is a string; expected a number", value)
	default:
		return nil, fmt.Errorf("%v is a %T; expected a number", propertyValue, propertyValue)
	}
}

func unmarshalProviderClusterConfiguration(pccYaml []byte, configObject map[string]any) (string, error) {
	if err := yaml.Unmarshal(pccYaml, &configObject); err != nil {
		return "", fmt.Errorf("yaml.Unmarshal: %w", err)
	}
	configKind, found, err := unstructured.NestedString(configObject, "kind")
	if err != nil {
		return "", fmt.Errorf("reading .kind: %w", err)
	}
	if !found {
		return "", fmt.Errorf("reading .kind: no such field")
	}
	return configKind, nil
}
