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
	isAutomatic := input.Values.Get("global.discovery.kubernetesVersionIsAutomatic").Bool()
	// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
	input.Logger.Info("E2E-KV istio-preflight",
		"source", "global.discovery.kubernetesVersionIsAutomatic",
		"isAutomatic", isAutomatic,
	)
	requirements.SaveValue(isK8sVersionAutomaticKey, isAutomatic)

	return nil
}
