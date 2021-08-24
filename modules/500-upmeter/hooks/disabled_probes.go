/*
Copyright 2021 Flant CJSC

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

type deploymentPresence struct {
	ccm, mcm, bashible, autoscaler bool
}

// collectDisabledProbes collects the references of probes (or probe groups) depending on enabled modules
// and deployed apps in the cluster
func collectDisabledProbes(input *go_hook.HookInput) error {
	// Parse input
	snapshot := parseSnapshotSet(input.Snapshots["deployments"])
	presence := deploymentPresence{
		ccm:        snapshot.has("cloud-controller-manager"),
		mcm:        snapshot.has("machine-controller-manager"),
		bashible:   snapshot.has("bashible-apiserver"),
		autoscaler: snapshot.has("cluster-autoscaler"),
	}
	enabledModules := parseValuesSet(input.Values, "global.enabledModules")
	disabledProbes := parseValuesSet(input.Values, "upmeter.disabledProbes")

	// Process the cluster state, `disabledProbes` is modified
	disableSyntheticProbes(input.Values, disabledProbes)
	disableMonitoringAndAutoscalingProbes(enabledModules, disabledProbes)
	disableScalingProbes(presence, enabledModules, disabledProbes)

	// Update the combined value of disabled probes
	input.Values.Set("upmeter.internal.disabledProbes", disabledProbes.slice())
	return nil
}

func disableSyntheticProbes(values *go_hook.PatchableValues, disabledProbes set) {
	if values.Get("upmeter.smokeMiniDisabled").Bool() {
		disabledProbes.add("synthetic/")
	}
}

func disableScalingProbes(presence deploymentPresence, enabledModules, disabledProbes set) {
	if !enabledModules.has("node-manager") {
		// The whole probe group is useless
		disabledProbes.add("scaling/")
		return
	}

	shouldScale := presence.ccm && presence.mcm && presence.bashible
	if !shouldScale {
		disabledProbes.add("scaling/cluster-scaling")
	}
	if !presence.autoscaler {
		disabledProbes.add("scaling/cluster-autoscaler")
	}
}

func disableMonitoringAndAutoscalingProbes(enabledModules, disabledProbes set) {
	// Disabling the whole group to simplify the env value for humans.
	if !enabledModules.has("prometheus") {
		disabledProbes.add("monitoring-and-autoscaling/")
		return
	}
	if !enabledModules.has("prometheus-metrics-adapter") {
		disabledProbes.add("monitoring-and-autoscaling/prometheus-metrics-adapter")
		disabledProbes.add("monitoring-and-autoscaling/horizontal-pod-autoscaler")
	}
	if !enabledModules.has("vertical-pod-autoscaler") {
		disabledProbes.add("monitoring-and-autoscaling/vertical-pod-autoscaler")
	}
	if !enabledModules.has("monitoring-kubernetes") {
		disabledProbes.add("monitoring-and-autoscaling/metrics-sources")
		disabledProbes.add("monitoring-and-autoscaling/key-metrics-present")
	}
}

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func parseSnapshotSet(results []go_hook.FilterResult) set {
	s := set{}
	for _, r := range results {
		s.add(r.(string))
	}
	return s
}

func parseValuesSet(values *go_hook.PatchableValues, path string) set {
	s := set{}
	for _, m := range values.Get(path).Array() {
		s.add(m.String())
	}
	return s
}

func newSet(xs ...string) set {
	s := set{}
	for _, x := range xs {
		s.add(x)
	}
	return s
}

type set map[string]struct{}

func (s set) add(x string) {
	s[x] = struct{}{}
}

func (s set) has(x string) bool {
	_, ok := s[x]
	return ok
}

func (s set) slice() []string {
	xs := make([]string, 0, len(s))
	for x := range s {
		xs = append(xs, x)
	}
	return xs
}

func (s set) delete(x string) set {
	delete(s, x)
	return s
}
