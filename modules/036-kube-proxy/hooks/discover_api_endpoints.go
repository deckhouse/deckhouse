/*
Copyright 2021 Flant JSC

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
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"golang.org/x/exp/slog"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/kube-proxy",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "kube_api_eps",
			ApiVersion:        "discovery.k8s.io/v1",
			Kind:              "EndpointSlice",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"default"}}},
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/service-name": "kubernetes"}},
			FilterFunc:        applyKubernetesAPIEndpointSliceFilter,
		},
	},
}, discoverAPIEndpointsHandler)

// KubernetesAPIEndpoints discovers kube api endpoints
type KubernetesAPIEndpoints struct {
	HostPort []string
}

func applyKubernetesAPIEndpointSliceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	slice := &discoveryv1.EndpointSlice{}
	err := sdk.FromUnstructured(obj, slice)
	if err != nil {
		return nil, err
	}

	mh := &KubernetesAPIEndpoints{}

	for _, p := range slice.Ports {
		for _, ep := range slice.Endpoints {
			for _, addr := range ep.Addresses {
				mh.HostPort = append(mh.HostPort, fmt.Sprintf("%s:%d", addr, *p.Port))
			}
		}
	}

	return mh, nil
}

func discoverAPIEndpointsHandler(_ context.Context, input *go_hook.HookInput) error {
	snapshots, err := sdkobjectpatch.UnmarshalToStruct[KubernetesAPIEndpoints](input.Snapshots, "kube_api_eps")
	if err != nil {
		return fmt.Errorf("failed to unmarshal kube_api_ep snapshot: %w", err)
	}

	if len(snapshots) == 0 {
		input.Logger.Error("EndpointSlices for kubernetes Service not found")
		return nil
	}
	seen := make(map[string]struct{})
	var allHostPort []string
	for _, snap := range snapshots {
		for _, hp := range snap.HostPort {
			if hp != "" {
				if _, exists := seen[hp]; !exists {
					seen[hp] = struct{}{}
					allHostPort = append(allHostPort, hp)
				}
			}
		}
	}

	if len(allHostPort) == 0 {
		return errors.New("no kubernetes apiserver endpoints host:port specified")
	}

	sort.Strings(allHostPort)

	input.Logger.Info("cluster master addresses", slog.String("addresses", strings.Join(allHostPort, ",")))

	input.Values.Set("kubeProxy.internal.clusterMasterAddresses", allHostPort)

	return nil
}
