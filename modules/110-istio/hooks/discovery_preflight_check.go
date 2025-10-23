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

package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	isK8sVersionAutomaticKey      = "istio:isK8sVersionAutomatic"
	istioToK8sCompatibilityMapKey = "istio:istioToK8sCompatibilityMap"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("istio-k8s-auto-discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "cluster-configuration",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        applyClusterConfigurationYamlFilter,
		},
	},
}, discoveryIsK8sVersionAutomatic)

func applyClusterConfigurationYamlFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	var metaConfig *config.MetaConfig
	// only cluster configuration, provider preparation and validation do not need here
	metaConfig, err = config.ParseConfigFromData(context.TODO(), string(ccYaml), config.DummyPreparatorProvider())
	if err != nil {
		return nil, err
	}

	kubernetesVersion, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
	if err != nil {
		return nil, err
	}

	return kubernetesVersion, err
}

func discoveryIsK8sVersionAutomatic(_ context.Context, input *go_hook.HookInput) error {
	var kubernetesVersionStr string
	clusterConfigurationSnapshots := input.Snapshots.Get("cluster-configuration")
	if len(clusterConfigurationSnapshots) == 0 {
		versionParts := strings.Split(input.Values.Get("global.discovery.kubernetesVersion").String(), ".")
		if len(versionParts) < 2 {
			return errors.New("cluster configuration kubernetesVersion is empty or invalid")
		}
		kubernetesVersionStr = versionParts[0] + "." + versionParts[1]
	} else {
		err := clusterConfigurationSnapshots[0].UnmarshalTo(&kubernetesVersionStr)
		if err != nil {
			return fmt.Errorf("failed to unmarshal 'cluster-configuration' snapshot: %w", err)
		}
	}

	// Get array of compatibility k8s versions for every operator version
	k8sCompatibleVersions := make(map[string][]string)
	_ = json.Unmarshal([]byte(input.Values.Get("istio.internal.istioToK8sCompatibilityMap").String()), &k8sCompatibleVersions)
	requirements.SaveValue(istioToK8sCompatibilityMapKey, k8sCompatibleVersions)

	requirements.SaveValue(isK8sVersionAutomaticKey, kubernetesVersionStr == "Automatic")

	return nil
}

func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	b, err := message.MarshalJSON()
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}
