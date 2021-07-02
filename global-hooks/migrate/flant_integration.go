// Copyright 2021 Flant CJSC
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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(flantIntegrationMigration))

func flantIntegrationMigration(input *go_hook.HookInput, dc dependency.Container) error {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %v", err)
	}

	cm, err := kubeCl.CoreV1().
		ConfigMaps("d8-system").
		Get(context.TODO(), "deckhouse", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf(`cannot get configmap "deckhouse"`)
	}

	// Second stage, we can delete other modules data
	if _, ok := cm.Data["flantIntegration"]; ok {
		input.LogEntry.Warn("flantIntegration already migrated, deleting other modules from the configmap")
		delete(cm.Data, "flantPricing")
		delete(cm.Data, "flantPricingEnabled")
		delete(cm.Data, "prometheusMadisonIntegration")
		delete(cm.Data, "prometheusMadisonIntegrationEnabled")

		// Do not retry on conflict, fail and start the hook one more time instead
		_, err := kubeCl.CoreV1().
			ConfigMaps("d8-system").
			Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf(`cannot update configmap "deckhouse"`)
		}
		return nil
	}

	input.LogEntry.Warn("flantIntegration migration started")

	// Copy settings from flantPricing as is
	settings := make(map[string]interface{})
	if pricingData, ok := cm.Data["flantPricing"]; ok {
		input.LogEntry.Warn(pricingData) // Backup through logs

		var parsedData map[string]interface{}
		err := yaml.Unmarshal([]byte(pricingData), &parsedData)
		if err == nil {
			for k, v := range parsedData {
				if k == "promscale" {
					k = "metrics"
				}
				settings[k] = v
			}
		}
	}

	// Copy only keys from prometheusMadisonIntegration settings and remove madison prefix
	if madisonData, ok := cm.Data["prometheusMadisonIntegration"]; ok {
		input.LogEntry.Warn(madisonData) // Backup through logs

		var parsedData map[string]interface{}
		err := yaml.Unmarshal([]byte(madisonData), &parsedData)
		if err == nil {
			for k, v := range parsedData {
				if k == "madisonAuthKey" {
					settings["madisonAuthKey"] = v
				}
			}
		}
	}

	if len(settings) > 0 {
		data, err := yaml.Marshal(settings)
		if err != nil {
			return err
		}

		cm.Data["flantIntegration"] = string(data)
	}

	flantPricingEnable, flantPricingEnableOk := cm.Data["flantPricingEnabled"]
	prometheusMadisonIntegrationEnabled := cm.Data["prometheusMadisonIntegrationEnabled"]

	if prometheusMadisonIntegrationEnabled == "false" || flantPricingEnable == "false" || !flantPricingEnableOk {
		if _, ok := cm.Data["flantIntegrationEnabled"]; !ok {
			cm.Data["flantIntegrationEnabled"] = "false"
		}
	}

	// Do not retry on conflict, fail and start the hook one more time instead
	_, err = kubeCl.CoreV1().
		ConfigMaps("d8-system").
		Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cannot update configmap deckhouse")
	}
	return nil
}
