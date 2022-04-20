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

package hooks

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	d8http "github.com/deckhouse/deckhouse/go_lib/dependency/http"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal"
	yandexV1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "exporter_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-yandex-cloud-metrics-exporter-app-creds"},
			},
			FilterFunc: applyMetricsExporterSecret,
		},
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, dependency.WithExternalDependencies(exporterAPIKeyHandler))

type apiKeySecret struct {
	Key                    string `json:"key"`
	KeyID                  string `json:"keyID"`
	ServiceAccountChecksum string `json:"serviceAccountChecksum"`
}

func applyProviderClusterConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	return secret, nil
}

func applyMetricsExporterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret = &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert secret from unstructured: %v", err)
	}

	apiKey := &apiKeySecret{
		Key: string(secret.Data["api-key"]),
	}

	annot := secret.GetAnnotations()
	if annot != nil {
		apiKey.ServiceAccountChecksum = annot["checksum/service-account"]
		apiKey.KeyID = annot["service-account-api-key/id"]
	}

	return apiKey, nil
}

func exporterAPIKeyHandler(input *go_hook.HookInput, dc dependency.Container) error {
	snapExporterSecret := input.Snapshots["exporter_secret"]
	var apiKey *apiKeySecret
	if len(snapExporterSecret) > 0 {
		apiKey = snapExporterSecret[0].(*apiKeySecret)
	}

	exporterEnabled := input.Values.Get("cloudProviderYandex.cloudMetricsExporterEnabled").Bool()
	if !exporterEnabled && apiKey == nil {
		// set everything to empty
		setAPIKeyValues(input, &apiKeySecret{})
		return nil
	}

	provider, err := parseProvider(input)
	if err != nil {
		return err
	}

	if !exporterEnabled && apiKey != nil {
		if apiKey.KeyID != "" {
			// should delete apiKey
			api, _, err := getAPI(provider.ServiceAccountJSON, dc.GetHTTPClient())
			if err != nil {
				return err
			}

			err = internal.WithRetry(3, 3*time.Second, func() error {
				return api.DeleteAPIKey(apiKey.KeyID)
			})

			if err != nil {
				return err
			}
		}

		// set everything to empty
		setAPIKeyValues(input, &apiKeySecret{})
		return nil
	}

	saCheckSumBytes := sha256.Sum256([]byte(provider.ServiceAccountJSON))
	saCheckSum := fmt.Sprintf("%x", saCheckSumBytes)

	if apiKey == nil || apiKey.Key == "" || saCheckSum != apiKey.ServiceAccountChecksum {
		api, saID, err := getAPI(provider.ServiceAccountJSON, dc.GetHTTPClient())
		if err != nil {
			return err
		}

		var newAPIKey, apiKeyID string
		err = internal.WithRetry(3, 3*time.Second, func() error {
			var err error
			newAPIKey, apiKeyID, err = api.CreateAPIKey(saID)
			return err
		})

		if err != nil {
			return err
		}

		apiKey = &apiKeySecret{
			Key:                    newAPIKey,
			ServiceAccountChecksum: saCheckSum,
			KeyID:                  apiKeyID,
		}
	}

	setAPIKeyValues(input, apiKey)

	return nil
}

func parseProvider(input *go_hook.HookInput) (*yandexV1.Provider, error) {
	snap := input.Snapshots["provider_cluster_configuration"]
	if len(snap) == 0 {
		return nil, fmt.Errorf("cannot find provide cluster configuration")
	}

	secret := snap[0].(*v1.Secret)
	var metaCfg *config.MetaConfig
	if clusterConfigurationYAML, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]; ok && len(clusterConfigurationYAML) > 0 {
		m, err := config.ParseConfigFromData(string(clusterConfigurationYAML))
		if err != nil {
			return nil, fmt.Errorf("validate cloud-provider-cluster-configuration.yaml error: %v", err)
		}
		metaCfg = m
	}

	providerConfRaw, ok := metaCfg.ProviderClusterConfig["provider"]
	if !ok {
		return nil, fmt.Errorf("provider section not present in provider cluster configuration")
	}

	var provider yandexV1.Provider

	err := json.Unmarshal(providerConfRaw, &provider)
	if err != nil {
		return nil, err
	}

	provider.ServiceAccountJSON = strings.TrimSpace(provider.ServiceAccountJSON)

	if provider.ServiceAccountJSON == "" {
		return nil, fmt.Errorf("service account is empty")
	}

	return &provider, nil
}

func setAPIKeyValues(input *go_hook.HookInput, apiKey *apiKeySecret) {
	input.Values.Set("cloudProviderYandex.internal.exporter.apiKey", apiKey.Key)
	input.Values.Set("cloudProviderYandex.internal.exporter.serviceAccountChecksum", apiKey.ServiceAccountChecksum)
	input.Values.Set("cloudProviderYandex.internal.exporter.apiKeyID", apiKey.KeyID)
}

func getAPI(serviceAccount string, client d8http.Client) (api *internal.YandexAPI, serviceAccountID string, err error) {
	var sa yandexV1.ServiceAccount
	err = json.Unmarshal([]byte(serviceAccount), &sa)
	if err != nil {
		return nil, "", err
	}

	api = internal.NewYandexAPI(client)
	err = api.Init(&sa)

	if err != nil {
		return nil, "", err
	}

	return api, sa.ServiceAccountID, nil
}
