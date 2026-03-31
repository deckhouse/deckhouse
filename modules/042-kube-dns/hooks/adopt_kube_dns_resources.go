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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var metadata = map[string]interface{}{
	"metadata": map[string]interface{}{
		"labels": map[string]string{
			"app.kubernetes.io/managed-by": "Helm",
			"app.kubernetes.io/instance":   "kube-dns",
		},
		"annotations": map[string]string{
			"meta.helm.sh/release-name":      "kube-dns",
			"meta.helm.sh/release-namespace": "d8-system",
		},
	},
}

func kubeDnsServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot create service object: %v", err)
	}

	return service.Spec.Type == v1.ServiceTypeClusterIP, nil
}

func deckhouseDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if err != nil || !found {
		return false, err
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		name, _, _ := unstructured.NestedString(container, "name")
		if name != "deckhouse" {
			continue
		}

		envs, found, err := unstructured.NestedSlice(container, "env")
		if err != nil || !found {
			return false, err
		}

		for _, e := range envs {
			env, ok := e.(map[string]interface{})
			if !ok {
				continue
			}

			if n, _, _ := unstructured.NestedString(env, "name"); n == "USE_NELM" {
				val, _, _ := unstructured.NestedString(env, "value")
				return val == "true", nil
			}
		}
	}

	return false, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},

	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_dns_svc",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kube-dns"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   kubeDnsServiceFilter,
		},
		{
			Name:       "deckhouse_deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   deckhouseDeploymentFilter,
		},
	},
}, adoptKubeDNSResources)

func adoptKubeDNSResources(_ context.Context, input *go_hook.HookInput) error {
	// Read USE_NELM flag from Deployment deckhouse
	useNelm := false
	if snap := input.Snapshots.Get("deckhouse_deployment"); len(snap) > 0 {
		if err := snap[0].UnmarshalTo(&useNelm); err != nil {
			return fmt.Errorf("cannot read USE_NELM from deckhouse Deployment: %w", err)
		}
	}

	// Deployment coredns must always be removed
	input.PatchCollector.DeleteNonCascading("apps/v1", "Deployment", "kube-system", "coredns")

	// If NELM is disabled — stop here (no adoption)
	if !useNelm {
		return nil
	}

	// Try to adopt Service kube-dns (only ClusterIP)
	snap := input.Snapshots.Get("kube_dns_svc")
	if len(snap) == 0 {
		return nil // nothing to adopt
	}

	var isClusterIP bool
	if err := snap[0].UnmarshalTo(&isClusterIP); err != nil {
		return fmt.Errorf("cannot determine kube-dns Service type: %w", err)
	}

	if !isClusterIP {
		return nil
	}

	// Apply Helm ownership metadata
	input.PatchCollector.PatchWithMerge(metadata, "v1", "Service", "kube-system", "kube-dns")

	return nil
}
