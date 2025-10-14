// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/filter"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	podSubnetByComponentSnapName = "pod_subnet_by_component"
	podSubnetByD8AppSnapName     = "pod_subnet_by_d8s_app"

	serviceSubnetByComponentSnapName = "service_subnet_by_component"
	serviceSubnetByD8AppSnapName     = "service_subnet_by_d8s_app"

	clusterConfigurationSnapName = "cluster_configuration"
)

var (
	podSubnetRegexp     = regexp.MustCompile(`(^|\s+)--cluster-cidr=([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\/[0-9]+)(\s+|$)`)
	serviceSubnetRegexp = regexp.MustCompile(`(^|\s+)--service-cluster-ip-range=([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+\/[0-9]+)(\s+|$)`)
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       podSubnetByComponentSnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "kube-controller-manager",
					"tier":      "control-plane",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyPodSubnetsFilter,
		},

		{
			Name:       podSubnetByD8AppSnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-controller-manager",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyPodSubnetsFilter,
		},

		{
			Name:       serviceSubnetByComponentSnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "kube-apiserver",
					"tier":      "control-plane",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyServiceSubnetsFilter,
		},

		{
			Name:       serviceSubnetByD8AppSnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-apiserver",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyServiceSubnetsFilter,
		},

		{
			Name:       clusterConfigurationSnapName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyClusterConfigurationFilter,
		},
	},
}, discoveryClusterIPRanges)

func applyClusterConfigurationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.Secret
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	clusterConf, ok := cm.Data["cluster-configuration.yaml"]
	if !ok {
		return "", nil
	}

	return clusterConf, nil
}

func applyPodSubnetsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return filter.GetArgFromUnstructuredPodWithRegexp(obj, podSubnetRegexp, 1, "kube-controller-manager")
}

func applyServiceSubnetsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return filter.GetArgFromUnstructuredPodWithRegexp(obj, serviceSubnetRegexp, 1, "kube-apiserver")
}

func getSubnetsFromSnapshots(input *go_hook.HookInput, snapshotsNames ...string) string {
	subnets := make([]string, 0)
	for _, s := range snapshotsNames {
		subnetsSnap, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, s)
		if err != nil {
			input.Logger.Warn("failed to unmarshal snapshot", slog.String("snapshot", s), log.Err(err))
			continue
		}
		if len(subnetsSnap) > 0 {
			subnets = append(subnets, subnetsSnap...)
		}
	}

	for _, subnet := range subnets {
		if subnet != "" {
			return subnet
		}
	}

	return ""
}

func discoveryClusterIPRanges(_ context.Context, input *go_hook.HookInput) error {
	clusterConfigSnap := input.Snapshots.Get(clusterConfigurationSnapName)
	if len(clusterConfigSnap) > 0 {
		return nil
	}

	podSubnet := getSubnetsFromSnapshots(input, podSubnetByComponentSnapName, podSubnetByD8AppSnapName)
	if podSubnet != "" {
		input.Values.Set("global.discovery.podSubnet", podSubnet)
	} else {
		input.Logger.Warn("Pod subnet not found")
	}

	serviceSubnet := getSubnetsFromSnapshots(input, serviceSubnetByComponentSnapName, serviceSubnetByD8AppSnapName)
	if serviceSubnet != "" {
		input.Values.Set("global.discovery.serviceSubnet", serviceSubnet)
	} else {
		input.Logger.Warn("Service subnet not found")
	}

	return nil
}
