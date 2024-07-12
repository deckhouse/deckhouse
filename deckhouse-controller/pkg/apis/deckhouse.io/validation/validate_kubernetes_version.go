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
	"encoding/base64"
	"fmt"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
)

func kubernetesVersionHandler() http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		if kubernetesversion.IsEnabled() {
			secret, ok := obj.(*v1.Secret)
			if !ok {
				return nil, fmt.Errorf("expect Secret as unstructured, got %T", obj)
			}
			val, ok := secret.Data["cluster-configuration.yaml"]
			if !ok {
				return nil, fmt.Errorf("expected field 'deckhouseDefaultKubernetesVersion' not found in secret %s", secret.Name)
			}
			clusterConfigurationRaw, err := base64.StdEncoding.DecodeString(string(val))
			if err != nil {
				return nil, err
			}
			var clusterConf struct {
				KubernetesVersion string `json:"kubernetesVersion"`
			}
			if err = yaml.Unmarshal(clusterConfigurationRaw, &clusterConf); err != nil {
				return nil, err
			}
			if err = kubernetesversion.Instance().ReverseValidate(clusterConf.KubernetesVersion); err != nil {
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
