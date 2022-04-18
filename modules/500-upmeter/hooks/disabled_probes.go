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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "deployments",
				ApiVersion: "apps/v1",
				Kind:       "Deployment",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{
						"d8-cloud-instance-manager",
						// CCM
						"d8-cloud-provider-aws",
						"d8-cloud-provider-azure",
						"d8-cloud-provider-gcp",
						"d8-cloud-provider-openstack",
						"d8-cloud-provider-vsphere",
						"d8-cloud-provider-yandex",
					}},
				},
				NameSelector: &types.NameSelector{MatchNames: []string{
					"bashible-apiserver",
					"cluster-autoscaler",
					"machine-controller-manager",
					"cloud-controller-manager",
				}},
				FilterFunc: filterName,
			},
		},
	},
	collectDisabledProbes,
)

type presenceInCluster struct {
	ccm, mcm, bashible, autoscaler, smokeMini bool
}

// collectDisabledProbes collects the references of probes (or probe groups) depending on enabled modules
// and deployed apps in the cluster
func collectDisabledProbes(input *go_hook.HookInput) error {
	// Parse input
	snapshot := set.NewFromSnapshot(input.Snapshots["deployments"])
	presence := presenceInCluster{
		ccm:        snapshot.Has("cloud-controller-manager"),
		mcm:        snapshot.Has("machine-controller-manager"),
		bashible:   snapshot.Has("bashible-apiserver"),
		autoscaler: snapshot.Has("cluster-autoscaler"),
		smokeMini:  !input.Values.Get("upmeter.smokeMiniDisabled").Bool(),
	}
	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")
	disabledProbes := set.NewFromValues(input.Values, "upmeter.disabledProbes")

	// Process the cluster state, `disabledProbes` is modified
	disableSyntheticProbes(presence, disabledProbes)
	disableMonitoringAndAutoscalingProbes(enabledModules, disabledProbes)
	disableScalingProbes(presence, enabledModules, disabledProbes)
	disableLoadBalancingProbes(presence, enabledModules, disabledProbes)

	// Update the combined value of disabled probes
	input.Values.Set("upmeter.internal.disabledProbes", disabledProbes.Slice())
	return nil
}

func disableSyntheticProbes(presence presenceInCluster, disabledProbes set.Set) {
	if !presence.smokeMini {
		disabledProbes.Add("synthetic/")
	}
}

func disableLoadBalancingProbes(presence presenceInCluster, enabledModules, disabledProbes set.Set) {
	if !enabledModules.Has("metallb") {
		disabledProbes.Add("load-balancing/metallb")
	}
	if !presence.ccm {
		disabledProbes.Add("load-balancing/load-balancer-configuration")
	}
}

func disableScalingProbes(presence presenceInCluster, enabledModules, disabledProbes set.Set) {
	if !enabledModules.Has("node-manager") {
		// The whole probe group is useless
		disabledProbes.Add("scaling/")
		return
	}

	shouldScale := presence.ccm && presence.mcm && presence.bashible
	if !shouldScale {
		disabledProbes.Add("scaling/cluster-scaling")
	}
	if !presence.autoscaler {
		disabledProbes.Add("scaling/cluster-autoscaler")
	}
}

func disableMonitoringAndAutoscalingProbes(enabledModules, disabledProbes set.Set) {
	// Disabling the whole group to simplify the env value for humans.
	if !enabledModules.Has("prometheus") {
		disabledProbes.Add("monitoring-and-autoscaling/")
		return
	}
	if !enabledModules.Has("prometheus-metrics-adapter") {
		disabledProbes.Add("monitoring-and-autoscaling/prometheus-metrics-adapter")
		disabledProbes.Add("monitoring-and-autoscaling/horizontal-pod-autoscaler")
	}
	if !enabledModules.Has("vertical-pod-autoscaler") {
		disabledProbes.Add("monitoring-and-autoscaling/vertical-pod-autoscaler")
	}
	if !enabledModules.Has("monitoring-kubernetes") {
		disabledProbes.Add("monitoring-and-autoscaling/metrics-sources")
		disabledProbes.Add("monitoring-and-autoscaling/key-metrics-present")
	}
}

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}
