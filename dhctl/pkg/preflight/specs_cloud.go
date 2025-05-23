// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"encoding/json"
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	minimumRequiredCPUCores           = 4
	minimumRequiredMemoryMB           = 8192 - reservedMemoryThresholdMB
	minimumRequiredRootDiskSizeGB     = 50
	minimumRequiredRegistryDiskSizeGB = 100

	reservedMemoryThresholdMB = 512
)

func (pc *Checker) CheckCloudMasterNodeSystemRequirements(_ context.Context) error {
	if app.PreflightSkipSystemRequirementsCheck {
		log.DebugLn("System requirements check is skipped")
		return nil
	}

	configObject := make(map[string]any)
	configKind, err := unmarshalProviderClusterConfiguration(pc.installConfig.ProviderClusterConfig, configObject)
	if err != nil {
		return fmt.Errorf("unmarshal provider cluster configuration: %v", err)
	}

	var coreCountPropertyPath, ramAmountPropertyPath, rootDiskPropertyPath, registryDiskPropertyPath []string
	switch configKind {
	case "AWSClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDisk", "sizeGb"}

	case "GCPClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "AzureClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "diskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "YandexClusterConfiguration":
		coreCountPropertyPath = []string{"masterNodeGroup", "instanceClass", "cores"}
		ramAmountPropertyPath = []string{"masterNodeGroup", "instanceClass", "memory"}
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "diskSizeGB"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "OpenStackClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSize"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "VsphereClusterConfiguration":
		coreCountPropertyPath = []string{"masterNodeGroup", "instanceClass", "numCPUs"}
		ramAmountPropertyPath = []string{"masterNodeGroup", "instanceClass", "memory"}
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSize"}

	case "VCDClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "ZvirtClusterConfiguration":
		coreCountPropertyPath = []string{"masterNodeGroup", "instanceClass", "numCPUs"}
		ramAmountPropertyPath = []string{"masterNodeGroup", "instanceClass", "memory"}
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}
		// externalDiskSizeDefault = 30

	case "DynamixClusterConfiguration":
		coreCountPropertyPath = []string{"masterNodeGroup", "instanceClass", "numCPUs"}
		ramAmountPropertyPath = []string{"masterNodeGroup", "instanceClass", "memory"}
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSizeGb"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}
		// externalDiskSizeDefault = 30

	case "HuaweiCloudClusterConfiguration":
		rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDiskSize"}
		registryDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "systemRegistryDiskSizeGb"}

	case "DVPClusterConfiguration":
		coreCountPropertyPath = []string{"masterNodeGroup", "instanceClass", "virtualMachine", "cpu", "cores"}
	// TODO: add checks for string values
	// ramAmountPropertyPath = []string{"masterNodeGroup", "instanceClass", "virtualMachine", "memory", "size"}
	// rootDiskPropertyPath = []string{"masterNodeGroup", "instanceClass", "rootDisk", "size"}

	default:
		return fmt.Errorf("unknown provider cluster configuration kind: %s", configKind)
	}

	if err = validateIntegerPropertyAtPath(configObject, rootDiskPropertyPath, minimumRequiredRootDiskSizeGB, true); err != nil {
		return fmt.Errorf("Root disk capacity: %v", err)
	}
	if err = validateIntegerPropertyAtPath(configObject, ramAmountPropertyPath, minimumRequiredMemoryMB, false); err != nil {
		return fmt.Errorf("RAM amount: %v", err)
	}
	if err = validateIntegerPropertyAtPath(configObject, coreCountPropertyPath, minimumRequiredCPUCores, false); err != nil {
		return fmt.Errorf("CPU cores count: %v", err)
	}
	if !pc.installConfig.Registry.IsDirect() {
		if err = validateIntegerPropertyAtPath(configObject, registryDiskPropertyPath, minimumRequiredRegistryDiskSizeGB, false); err != nil {
			return fmt.Errorf("Registry disk capacity: %v", err)
		}
	}

	return nil
}

func (pc *Checker) CheckSystemRegistryModuleSupport(_ context.Context) error {
	var cloud config.ClusterConfigCloudSpec
	if err := json.Unmarshal(pc.metaConfig.ClusterConfig["cloud"], &cloud); err != nil {
		return fmt.Errorf("unable to unmarshal cloud section from provider cluster configuration: %v", err)
	}

	if err := pc.metaConfig.ProviderSecondaryDevicesConfig.ValidateRegistryDataDevice(cloud.Provider); err != nil {
		return fmt.Errorf(
			"the module 'system-registry' is not supported with the cloud provider '%s': %v",
			cloud.Provider,
			err,
		)
	}
	return nil
}

func validateIntegerPropertyAtPath(configObject map[string]any, propertyPath []string, minimalValue int, allowMissing bool) error {
	if len(propertyPath) == 0 {
		return nil
	}

	propertyValue, found, err := unstructured.NestedFieldNoCopy(configObject, propertyPath...)
	if err != nil {
		return fmt.Errorf("malformed provider cluster configuration: reading .%s: %w", strings.Join(propertyPath, "."), err)
	}
	if !found {
		if allowMissing {
			return nil
		}
		return fmt.Errorf("malformed provider cluster configuration: reading .%s: no such property", strings.Join(propertyPath, "."))
	}

	if propertyValue.(int) < minimalValue {
		return fmt.Errorf("expected at least %d, but %d is configured", minimalValue, propertyValue)
	}

	return nil
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
