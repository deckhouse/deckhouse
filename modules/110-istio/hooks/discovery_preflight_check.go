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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	isK8sVersionAutomaticKey      = "istio:isK8sVersionAutomatic"
	istioToK8sCompatibilityMapKey = "istio:istioToK8sCompatibilityMap"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        lib.Queue("istio-k8s-auto-discovery"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(discoveryIsK8sVersionAutomatic))

func discoveryIsK8sVersionAutomatic(_ context.Context, input *go_hook.HookInput, _ dependency.Container) error {
	k8sCompatibleVersions := make(map[string][]string)
	if err := json.Unmarshal([]byte(input.Values.Get("istio.internal.istioToK8sCompatibilityMap").String()), &k8sCompatibleVersions); err != nil {
		return fmt.Errorf("cannot parse istioToK8sCompatibilityMap: %w", err)
	}
	requirements.SaveValue(istioToK8sCompatibilityMapKey, k8sCompatibleVersions)

	// No ClusterConfiguration — a managed cluster: the provider owns the Kubernetes version. The gate
	// this key feeds compares the *coming* Deckhouse default with the installed Istio versions, which
	// is meaningless there and would block updates over a version the cluster will never run.
	//
	// Same answer as before by another route: global.clusterConfiguration is published from the very
	// Secret this hook used to read (global-hooks/discovery/cluster_configuration.go), and with that
	// Secret absent the old code fell through to the real cluster version, never "Automatic".
	if !input.Values.Get("global.clusterConfiguration").Exists() {
		requirements.SaveValue(isK8sVersionAutomaticKey, false)
		return nil
	}

	// Fail rather than assume "pinned": requirements/check.go treats false as "gate not applicable",
	// so a wrong false silently skips the Istio↔Kubernetes compatibility check on upgrades.
	// kubernetesVersionIsDefault defaults to false in the schema, so it cannot tell "pinned" from
	// "not published yet" — targetKubernetesVersion has no default, and its emptiness can.
	if input.Values.Get("global.discovery.targetKubernetesVersion").String() == "" {
		return fmt.Errorf("cannot determine whether the Kubernetes version is pinned: " +
			"global.discovery.targetKubernetesVersion is empty")
	}

	isAutomatic := input.Values.Get("global.discovery.kubernetesVersionIsDefault").Bool()
	requirements.SaveValue(isK8sVersionAutomaticKey, isAutomatic)

	return nil
}
