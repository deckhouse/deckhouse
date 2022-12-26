/*
Copyright 2022 Flant JSC

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

// TODO remove after 1.42 release

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const DataKey = "cluster-configuration.yaml"

type packagesProxy struct {
	URI      string `json:"uri" yaml:"uri"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(clusterConfigurationMigration))

func clusterConfigurationMigration(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	secret, err := kubeCl.CoreV1().
		Secrets("kube-system").
		Get(context.TODO(), "d8-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		input.LogEntry.Info("cannot get Secret/kube-system/d8-cluster-configuration, proxy configuration migration skipped")
		return nil
	}

	configYaml, ok := secret.Data[DataKey]
	if !ok {
		input.LogEntry.Warnf("key %q not found in Secret/kube-system/d8-cluster-configuration", DataKey)
		return nil
	}

	input.LogEntry.Info(string(configYaml)) // Backup through logs

	config := make(map[string]interface{})
	if err := yaml.Unmarshal(configYaml, &config); err != nil {
		return fmt.Errorf("cannot unmarshal %s config: %v", DataKey, err)
	}

	if _, ok := config["proxy"]; ok {
		input.LogEntry.Info("proxy parameter is set, migration is not needed")
		return nil
	}

	var needMigration bool

	if val, ok := config["packagesProxy"]; ok {
		var packagesProxy packagesProxy

		needMigration = true

		pp, err := json.Marshal(val)
		if err != nil {
			return err
		}

		err = json.Unmarshal(pp, &packagesProxy)
		if err != nil {
			return err
		}

		var authInfo string
		if packagesProxy.Username != "" {
			authInfo = packagesProxy.Username
		}
		if packagesProxy.Password != "" {
			authInfo = authInfo + ":" + packagesProxy.Password
		}

		proxyString := packagesProxy.URI
		reg := regexp.MustCompile(`^(https?://)`)
		if !reg.MatchString(packagesProxy.URI) {
			return fmt.Errorf("packagesProxy.uri should start from `http[s]://`: %s", packagesProxy.URI)
		}

		if authInfo != "" {
			proxyString = reg.ReplaceAllString(packagesProxy.URI, "${1}"+authInfo+"@")
		}

		delete(config, "packagesProxy")
		config["proxy"] = map[string]interface{}{
			"httpProxy":  proxyString,
			"httpsProxy": proxyString,
		}
	}

	if resJSON, ok := input.ConfigValues.GetOk("global.modules.proxy"); ok {
		needMigration = true
		res := make(map[string]interface{})
		err := json.Unmarshal([]byte(resJSON.String()), &res)
		if err != nil {
			return err
		}
		config["proxy"] = res
	}

	if !needMigration {
		input.LogEntry.Info("migration is not needed")
		return nil
	}

	configYaml, err = yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("cannot marshal %s config: %v", DataKey, err)
	}

	secret.Data[DataKey] = configYaml

	_, err = kubeCl.CoreV1().
		Secrets("kube-system").
		Update(context.TODO(), secret, metav1.UpdateOptions{})

	return err
}
