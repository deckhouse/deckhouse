/*
Copyright 2025 Flant JSC

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
	"log/slog"
	"net/http"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	providerClusterConfigurationSecretName = "d8-provider-cluster-configuration"
	staticClusterConfigurationSecretName   = "d8-static-cluster-configuration"

	allowDeleteAnnotationKey = "deckhouse.io/allow-delete"
)

var secretNameToConfig = map[string]string{
	providerClusterConfigurationSecretName: "cloud-provider-cluster-configuration.yaml",
	staticClusterConfigurationSecretName:   "static-cluster-configuration.yaml",
}

func validateClusterConfigurationDeletion(obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
	name := obj.GetName()

	if name == providerClusterConfigurationSecretName {
		if _, ok := obj.GetAnnotations()[allowDeleteAnnotationKey]; ok {
			return allowResult(nil)
		}

		return rejectResult(fmt.Sprintf(
			"It is forbidden to delete secret %s. Please annotate Secret with `%s=true` if you're sure that you want to delete the secret.",
			providerClusterConfigurationSecretName, allowDeleteAnnotationKey,
		))
	}

	if name == staticClusterConfigurationSecretName {
		return rejectResult(fmt.Sprintf(
			"It is forbidden to delete secret %s",
			staticClusterConfigurationSecretName,
		))
	}

	return allowResult(nil)
}

func providerAndStaticConfigurationHandler(schemaStore *config.SchemaStore) http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, ar *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		dataKey, ok := secretNameToConfig[obj.GetName()]
		if !ok {
			return allowResult(nil)
		}

		switch ar.Operation {
		case model.OperationDelete:
			return validateClusterConfigurationDeletion(obj)

		default:
			secret, ok := obj.(*v1.Secret)
			if !ok {
				log.Debug("unexpected type", log.Type("expected", v1.Secret{}), log.Type("got", obj))
				return nil, fmt.Errorf("expect Secret as unstructured, got %T", obj)
			}

			clusterConfigurationRaw, ok := secret.Data[dataKey]
			if !ok {
				log.Debug(
					"no cluster-configuration found in secret",
					slog.String("namespace", obj.GetNamespace()), slog.String("name", obj.GetName()),
				)
				return nil, fmt.Errorf("expected field '%s' not found in secret %s", dataKey, secret.Name)
			}

			return validateClusterConfiguration(schemaStore, clusterConfigurationRaw)
		}
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "provider-and-static-configuration-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1.Secret{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
