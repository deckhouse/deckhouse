// Copyright 2022 Flant JSC
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

package hooks

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(ciliumModeMigration))

func ciliumModeMigration(input *go_hook.HookInput, dc dependency.Container) error {
	// Setup
	configMigrator, err := newModuleConfigMigrator(dc, input)
	if err != nil {
		return err
	}
	const cmKey = "cniCilium"

	// Get config
	config, err := configMigrator.getConfig(cmKey)
	if err != nil {
		return err
	}

	// If cilium type is set, exit
	if _, ok := config["type"]; ok {
		return nil
	}

	// Get cloud-provider node-routes
	nodeRoutes, err := isNodeRoutesNeeded(input, dc)
	if err != nil {
		return err
	}

	// Get createNodeRoutes from cm and if it set, override nodeRoutes
	if nr, ok := config["createNodeRoutes"]; ok {
		nodeRoutes = nr.(bool)
	}
	delete(config, "createNodeRoutes")

	ciliumMode := "Direct"
	if tunnelMode, ok := config["tunnelMode"].(string); ok && tunnelMode == "VXLAN" {
		ciliumMode = "VXLAN"
	}
	delete(config, "tunnelMode")

	if nodeRoutes && ciliumMode == "Direct" {
		ciliumMode = "DirectWithNodeRoutes"
	}

	// Migrate
	if config == nil {
		config = make(map[string]interface{})
	}

	if ciliumMode != "Direct" {
		config["mode"] = ciliumMode
	}
	// Save config
	return configMigrator.setConfig(cmKey, config)
}

func isNodeRoutesNeeded(input *go_hook.HookInput, dc dependency.Container) (bool, error) {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return false, fmt.Errorf("cannot init Kubernetes client: %v", err)
	}
	secret, err := kubeCl.CoreV1().
		Secrets("kube-system").
		Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})

	if err != nil {
		return false, fmt.Errorf("cannot get Secret/kube-system/d8-cluster-configuration: %v", err)
	}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return false, fmt.Errorf("cannot get `cluster-configuration.yaml field from secret`")
	}

	metaConfig, err := config.ParseConfigFromData(string(ccYaml))
	if err != nil {
		return false, err
	}

	if metaConfig.ClusterType == config.StaticClusterType {
		input.LogEntry.Info("static cluster detected")
		return true, nil
	}

	input.LogEntry.Infof("cloud-provider %s detected", metaConfig.ProviderName)
	switch strings.ToLower(metaConfig.ProviderName) {
	case "openstack", "vsphere":
		return true, nil
	}
	// Another cloud-provider
	return false, nil
}
