/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 20,
	},
}, dependency.WithExternalDependencies(handleGlobalValuesAndKubectl))

func handleGlobalValuesAndKubectl(input *go_hook.HookInput, dc dependency.Container) error {
	var (
		cloudProvider           = "none"
		controlPlaneVersion     *semver.Version
		clusterType             = "Cloud"
		terraformManagerEnabled bool
	)

	modules := input.Values.Get("global.enabledModules").Array()
	if modules == nil {
		return fmt.Errorf("got nil global.enabledModules")
	}
	for _, module := range modules {
		moduleName := module.String()
		if strings.HasPrefix(moduleName, "cloud-provider-") {
			cloudProvider = strings.TrimPrefix(moduleName, "cloud-provider-")
		}
		if moduleName == "terraform-manager" {
			terraformManagerEnabled = true
		}
	}

	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}
	version, err := k8.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("can't get Kubernetes version: %v", err)
	}
	serverVersion := version.String()
	controlPlaneVersion, err = semver.NewVersion(serverVersion[1:])
	if err != nil {
		return fmt.Errorf("can't parse Kubernetes version: %v", err)
	}

	if input.Values.Exists("global.clusterConfiguration") {
		clusterType = input.Values.Get("global.clusterConfiguration.clusterType").String()
		staticNodesCount, ok := input.Values.GetOk("flantIntegration.internal.nodeStats.staticNodesCount")
		if !ok {
			return fmt.Errorf("waiting for `internal.nodeStats.staticNodesCount` to be defined")
		}
		if (clusterType == "Static" && cloudProvider != "none") || (clusterType == "Cloud" && staticNodesCount.Int() > 0) {
			clusterType = "Hybrid"
		}
	}

	if input.Values.Exists("flantIntegration.clusterType") {
		clusterType = input.Values.Get("flantIntegration.clusterType").String()
	}

	input.Values.Set("flantIntegration.internal.cloudProvider", cloudProvider)
	input.Values.Set("flantIntegration.internal.controlPlaneVersion",
		fmt.Sprintf("%d.%d", controlPlaneVersion.Major(), controlPlaneVersion.Minor()))

	input.Values.Set("flantIntegration.internal.clusterType", clusterType)
	input.Values.Set("flantIntegration.internal.terraformManagerEnabled", terraformManagerEnabled)

	return nil
}
