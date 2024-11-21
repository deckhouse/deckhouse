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

package validation

import (
	"context"
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type clusterConfig struct {
	KubernetesVersion string `json:"kubernetesVersion"`
}

func kubernetesVersionHandler(mm moduleManager) http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		secret, ok := obj.(*v1.Secret)
		if !ok {
			log.Debugf("unexpected type, expected %T, got %T", v1.Secret{}, obj)
			return nil, fmt.Errorf("expect Secret as unstructured, got %T", obj)
		}

		clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
		if !ok {
			log.Debugf("no cluster-configuration found in secret %s/%s", obj.GetNamespace(), obj.GetName())
			return nil, fmt.Errorf("expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
		}

		clusterConf := new(clusterConfig)
		if err := yaml.Unmarshal(clusterConfigurationRaw, clusterConf); err != nil {
			log.Debugf("failed to unmarshal cluster configuration: %v", err)
			return nil, err
		}

		if clusterConf.KubernetesVersion == "Automatic" {
			clusterConf.KubernetesVersion = config.DefaultKubernetesVersion
		}

		if moduleName, err := kubernetesversion.Instance().ValidateBaseVersion(clusterConf.KubernetesVersion); err != nil {
			log.Debugf("failed to validate base version: %v", err)
			if moduleName == "" {
				return rejectResult(err.Error())
			}
			if mm.IsModuleEnabled(moduleName) {
				log.Debugf("module %s has unsatisfied requierements", moduleName)
				return rejectResult(err.Error())
			}
		}

		return allowResult("")
	})

	// Create webhook.
	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "kubernetes-version-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1.Secret{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
