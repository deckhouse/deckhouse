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

	// No ClusterConfiguration means Deckhouse does not own the Kubernetes version — a managed
	// cluster, where control-plane-manager is disabled and the provider decides the version. The
	// requirement gated by this key compares the *coming Deckhouse release's default* Kubernetes
	// version with the installed Istio versions, which is meaningless there and would block
	// Deckhouse updates over a version the cluster will never run.
	//
	// This reproduces what the ClusterConfiguration Secret answered before the version moved: with
	// the Secret absent the hook fell through to the actual cluster version, which never equals the
	// literal "Automatic", so the requirement was always skipped. global.clusterConfiguration is
	// published from exactly that Secret (global-hooks/discovery/cluster_configuration.go) and stays
	// absent while the Secret is, so its presence is the same signal by a different route.
	//
	// The one case where the old code answered differently is one its schema made unreachable:
	// kubernetesVersion was required in ClusterConfiguration, so "Secret present, nothing pinned
	// anywhere" could not occur. It can now, and "Deckhouse picks the version" is precisely what
	// this key is meant to report, so the bool below is the right answer for it.
	if !input.Values.Get("global.clusterConfiguration").Exists() {
		requirements.SaveValue(isK8sVersionAutomaticKey, false)
		return nil
	}

	// Fail rather than assume "pinned". requirements/check.go treats false as "gate not applicable"
	// and returns true, so a false published here silently skips the Istio↔Kubernetes compatibility
	// check on Deckhouse upgrades. The previous implementation read the ClusterConfiguration Secret
	// directly and errored when it could not determine the version; reading a bool whose schema
	// default is false lost that fail-closed behaviour.
	//
	// targetKubernetesVersion is the tell: it has no schema default, so an empty value means the
	// global discovery hook has not published yet (or failed) and the bool cannot be trusted.
	if input.Values.Get("global.discovery.targetKubernetesVersion").String() == "" {
		return fmt.Errorf("cannot determine whether the Kubernetes version is pinned: " +
			"global.discovery.targetKubernetesVersion is empty")
	}

	isAutomatic := input.Values.Get("global.discovery.kubernetesVersionIsDefault").Bool()
	requirements.SaveValue(isK8sVersionAutomaticKey, isAutomatic)

	return nil
}
