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

package smokemini

import (
	"encoding/json"
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/scheduler"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

const (
	Namespace = "d8-upmeter"
)

var (
	namespaceSelector = &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{Namespace}}}
	labelSelector     = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "smoke-mini"}}
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Schedule: []go_hook.ScheduleConfig{{
			Name:    "reschedule",
			Crontab: "* * * * *",
		}},
		Queue: "/modules/upmeter/update_selector",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                         "nodes",
				ApiVersion:                   "v1",
				Kind:                         "Node",
				FilterFunc:                   snapshot.NewNode,
				ExecuteHookOnSynchronization: pointer.Bool(false),
				WaitForSynchronization:       pointer.Bool(false),
			},
			{
				Name:              "statefulsets",
				ApiVersion:        "apps/v1",
				Kind:              "StatefulSet",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewStatefulSet,

				ExecuteHookOnEvents:          pointer.Bool(false),
				ExecuteHookOnSynchronization: pointer.Bool(false),
				WaitForSynchronization:       pointer.Bool(false),
			},
			{
				Name:              "pods",
				ApiVersion:        "v1",
				Kind:              "Pod",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPod,

				ExecuteHookOnEvents:          pointer.Bool(false),
				ExecuteHookOnSynchronization: pointer.Bool(false),
				WaitForSynchronization:       pointer.Bool(false),
			},
			{
				Name:              "pdb",
				ApiVersion:        "policy/v1",
				Kind:              "PodDisruptionBudget",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewDisruption,

				ExecuteHookOnEvents:          pointer.Bool(false),
				ExecuteHookOnSynchronization: pointer.Bool(false),
				WaitForSynchronization:       pointer.Bool(false),
			},
			{
				Name:                         "default_sc",
				ApiVersion:                   "storage.k8s.io/v1",
				Kind:                         "StorageClass",
				FilterFunc:                   snapshot.NewStorageClass,
				ExecuteHookOnSynchronization: pointer.Bool(false),
				WaitForSynchronization:       pointer.Bool(false),
			},
			{
				Name:              "pvc",
				ApiVersion:        "v1",
				Kind:              "PersistentVolumeClaim",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPvcTermination,

				ExecuteHookOnSynchronization: pointer.Bool(false),
			},
		},
	},
	reschedule,
)

func reschedule(input *go_hook.HookInput) error {
	if !smokeMiniEnabled(input.Values) {
		return nil
	}

	logger := input.LogEntry
	const statePath = "upmeter.internal.smokeMini.sts"

	// Parse the state from values
	statefulSets := snapshot.ParseStatefulSetSlice(input.Snapshots["statefulsets"])
	state, err := parseState(input.Values.Get(statePath))
	if err != nil {
		return err
	}

	if state.Empty() && len(statefulSets) > 0 {
		logger.Info(`Smoke-mini state is empty while statefulsets exist. Skipping until values are filled by "scrape_state.go" hook.`)
		return nil
	}

	var (
		// Parse inputs
		storageClass = getSmokeMiniStorageClass(input.Values, input.Snapshots["default_sc"])
		image        = getSmokeMiniImage(input.Values)

		nodes             = snapshot.ParseNodeSlice(input.Snapshots["nodes"])
		pods              = snapshot.ParsePodSlice(input.Snapshots["pods"])
		pvcs              = snapshot.ParsePvcTerminationSlice(input.Snapshots["pvc"])
		disruptionAllowed = parseAllowedDisruption(input.Snapshots["pdb"])

		// Construct
		stsSelector  = scheduler.NewStatefulSetSelector(nodes, storageClass, pvcs, pods, disruptionAllowed)
		nodeSelector = scheduler.NewNodeSelector(state)
		kubeCleaner  = scheduler.NewCleaner(input.PatchCollector, logger, pods)
		sched        = scheduler.New(stsSelector, nodeSelector, kubeCleaner, image, storageClass)
	)

	// Do the job
	x, newSts, err := sched.Schedule(state, nodes)
	if err != nil {
		if errors.Is(err, scheduler.ErrSkip) {
			logger.Info(err)
			return nil
		}
		return err
	}

	// Update values
	state[x] = newSts
	input.Values.Set(statePath, state)
	return nil
}

// parseState parses the state from values
func parseState(stateValues gjson.Result) (scheduler.State, error) {
	var state scheduler.State
	err := json.Unmarshal([]byte(stateValues.Raw), &state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func getK8sDefaultStorageClass(rs []go_hook.FilterResult) string {
	parsed := snapshot.ParseStorageClassSlice(rs)
	for _, sc := range parsed {
		if sc.Default {
			return sc.Name
		}
	}
	return ""
}

func parseAllowedDisruption(rs []go_hook.FilterResult) bool {
	allowances := parseBoolSnapshot(rs)
	if len(allowances) == 0 {
		// No PDB means any disruption allowed. Smoke-mini PDB could have been deleted on purpose.
		return true
	}
	return allowances[0]
}

// parseBoolSnapshot parses bool from snapshots
func parseBoolSnapshot(rs []go_hook.FilterResult) []bool {
	ret := make([]bool, len(rs))
	for i, r := range rs {
		ret[i] = r.(bool)
	}
	return ret
}

func getSmokeMiniImage(values *go_hook.PatchableValues) string {
	var (
		registry = values.Get("global.modulesImages.registry.base").String()
		digest   = values.Get("global.modulesImages.digests.upmeter.smokeMini").String()
	)
	return registry + "@" + digest
}

func getSmokeMiniStorageClass(values *go_hook.PatchableValues, storageClassSnap []go_hook.FilterResult) string {
	var (
		k8s = getK8sDefaultStorageClass(storageClassSnap)
		d8  = values.Get("global.storageClass").String()
		sm  = values.Get("upmeter.smokeMini.storageClass").String()
	)
	return firstNonEmpty(sm, d8, k8s, snapshot.DefaultStorageClass)
}

// firstNonEmpty returns first non-empty string. Returns empty string if no strings passed, or all
// arguments are empty strings.
func firstNonEmpty(xs ...string) string {
	for _, s := range xs {
		if s != "" {
			return s
		}
	}
	return ""
}

// smokeMiniEnabled returns true if smoke-mini is not disabled. This function is to avoid reversed
// boolean naming.
func smokeMiniEnabled(v *go_hook.PatchableValues) bool {
	disabled := v.Get("upmeter.smokeMiniDisabled").Bool()
	return !disabled
}
