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

type FilterResult struct {
	IsClusterIP bool
	IsUseNelm   bool
}

func kubeDNSServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot create service object: %v", err)
	}

	return FilterResult{IsClusterIP: service.Spec.Type == v1.ServiceTypeClusterIP}, nil
}

func deckhouseDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	containers, _, _ := unstructured.NestedSlice(
		obj.Object,
		"spec", "template", "spec", "containers",
	)

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		if name, _, _ := unstructured.NestedString(container, "name"); name == "deckhouse" {
			envs, _, _ := unstructured.NestedSlice(container, "env")

			for _, e := range envs {
				env, ok := e.(map[string]interface{})
				if !ok {
					continue
				}

				if n, _, _ := unstructured.NestedString(env, "name"); n == "USE_NELM" {
					val, _, _ := unstructured.NestedString(env, "value")

					return FilterResult{
						IsUseNelm: val == "true",
					}, nil
				}
			}
		}
	}

	return FilterResult{}, nil
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
			FilterFunc:                   kubeDNSServiceFilter,
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
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   deckhouseDeploymentFilter,
		},
	},
}, adoptKubeDNSResources)

func adoptKubeDNSResources(_ context.Context, input *go_hook.HookInput) error {
	var deckhouseDeploymentResult FilterResult
	var kubeDNSServiceResult FilterResult

	if snap := input.Snapshots.Get("deckhouse_deployment"); len(snap) > 0 {
		if err := snap[0].UnmarshalTo(&deckhouseDeploymentResult); err != nil {
			return fmt.Errorf("cannot get USE_NELM from Deployment deckhouse: %w", err)
		}
	}

	if snap := input.Snapshots.Get("kube_dns_svc"); len(snap) > 0 {
		if err := snap[0].UnmarshalTo(&kubeDNSServiceResult); err != nil {
			return fmt.Errorf("cannot get service type from Service kube-dns: %w", err)
		}
	}

	// Always remove Deployment coredns
	input.PatchCollector.DeleteNonCascading("apps/v1", "Deployment", "kube-system", "coredns")

	// If service is not ClusterIP, nothing else to do
	if !kubeDNSServiceResult.IsClusterIP {
		return nil
	}

	// If NELM disabled → delete Service
	if !deckhouseDeploymentResult.IsUseNelm {
		input.PatchCollector.Delete("v1", "Service", "kube-system", "kube-dns")
		return nil
	}

	// Otherwise adopt Service
	input.PatchCollector.PatchWithMerge(metadata, "v1", "Service", "kube-system", "kube-dns")

	return nil
}
