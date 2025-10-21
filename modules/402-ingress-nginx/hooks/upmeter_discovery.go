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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// This hook discovers conrtoller names for dynamic probes in upmeter
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ingress-nginx",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress_nginx_controllers",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "IngressNginxController",
			FilterFunc: filterName,
		},
	},
}, collectDynamicProbeConfig)

type upmeterDiscovery struct {
	ControllerNames []string `json:"controllerNames"`
}

// collectDynamicProbeConfig sets names of objects to internal values
func collectDynamicProbeConfig(_ context.Context, input *go_hook.HookInput) error {
	// Input
	key := "ingressNginx.internal.upmeterDiscovery"
	names, err := parseNames(input.Snapshots.Get("ingress_nginx_controllers"))
	if err != nil {
		return fmt.Errorf("failed to parse names: %w", err)
	}

	discovery := upmeterDiscovery{
		ControllerNames: names,
	}

	// Output
	input.Values.Set(key, discovery)
	return nil
}

// parseNames parses filter string result to a sorted strings slice
func parseNames(results []pkg.Snapshot) ([]string, error) {
	s := set.New()
	for name, err := range sdkobjectpatch.SnapshotIter[string](results) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'ingress_nginx_controllers' snapshots: %w", err)
		}
		s.Add(name)
	}
	s.Delete("") // throw away invalid ones
	return s.Slice(), nil
}

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}
