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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/scheduler"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
	"github.com/deckhouse/deckhouse/pkg/log"
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
				ExecuteHookOnSynchronization: ptr.To(false),
				WaitForSynchronization:       ptr.To(false),
			},
			{
				Name:              "statefulsets",
				ApiVersion:        "apps/v1",
				Kind:              "StatefulSet",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewStatefulSet,

				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				WaitForSynchronization:       ptr.To(false),
			},
			{
				Name:              "pods",
				ApiVersion:        "v1",
				Kind:              "Pod",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPod,

				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				WaitForSynchronization:       ptr.To(false),
			},
			{
				Name:              "pdb",
				ApiVersion:        "policy/v1",
				Kind:              "PodDisruptionBudget",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewDisruption,

				ExecuteHookOnEvents:          ptr.To(false),
				ExecuteHookOnSynchronization: ptr.To(false),
				WaitForSynchronization:       ptr.To(false),
			},
			{
				Name:                         "default_sc",
				ApiVersion:                   "storage.k8s.io/v1",
				Kind:                         "StorageClass",
				FilterFunc:                   snapshot.NewStorageClass,
				ExecuteHookOnSynchronization: ptr.To(false),
				WaitForSynchronization:       ptr.To(false),
			},
			{
				Name:              "pvc",
				ApiVersion:        "v1",
				Kind:              "PersistentVolumeClaim",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPvcTermination,

				ExecuteHookOnSynchronization: ptr.To(false),
			},
		},
	},
	reschedule,
)

func reschedule(_ context.Context, input *go_hook.HookInput) error {
	if !smokeMiniEnabled(input.Values) {
		return nil
	}

	logger := input.Logger
	const statePath = "upmeter.internal.smokeMini.sts"

	// Parse the state from values
	statefulSets, err := snapshot.ParseStatefulSetSlice(input.Snapshots.Get("statefulsets"))
	if err != nil {
		return err
	}

	state, err := parseState(input.Values.Get(statePath))
	if err != nil {
		return err
	}

	if state.Empty() && len(statefulSets) > 0 {
		logger.Info(`Smoke-mini state is empty while statefulsets exist. Skipping until values are filled by "scrape_state.go" hook.`)
		return nil
	}

	// Parse inputs
	storageClass, err := getSmokeMiniStorageClass(input.Values, input.Snapshots.Get("default_sc"))
	if err != nil {
		return err
	}

	image := getSmokeMiniImage(input.Values)

	nodes, err := snapshot.ParseNodeSlice(input.Snapshots.Get("nodes"))
	if err != nil {
		return err
	}

	pods, err := snapshot.ParsePodSlice(input.Snapshots.Get("pods"))
	if err != nil {
		return err
	}

	pvcs, err := snapshot.ParsePvcTerminationSlice(input.Snapshots.Get("pvc"))
	if err != nil {
		return err
	}

	disruptionAllowed, err := parseAllowedDisruption(input.Snapshots.Get("pdb"))
	if err != nil {
		return err
	}

	// Construct
	stsSelector := scheduler.NewStatefulSetSelector(nodes, storageClass, pvcs, pods, disruptionAllowed)
	nodeSelector := scheduler.NewNodeSelector(state)
	kubeCleaner := scheduler.NewCleaner(input.PatchCollector, logger, pods)
	sched := scheduler.New(stsSelector, nodeSelector, kubeCleaner, image, storageClass)

	// Do the job
	x, newSts, err := sched.Schedule(state, nodes)
	if err != nil {
		if errors.Is(err, scheduler.ErrSkip) {
			logger.Info("scheduler skip", log.Err(err))
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

func getK8sDefaultStorageClass(rs []sdkpkg.Snapshot) (string, error) {
	parsed, err := snapshot.ParseStorageClassSlice(rs)
	if err != nil {
		return "", err
	}
	for _, sc := range parsed {
		if sc.Default {
			return sc.Name, nil
		}
	}
	return "", nil
}

func parseAllowedDisruption(rs []sdkpkg.Snapshot) (bool, error) {
	if len(rs) == 0 {
		// No PDB means any disruption allowed. Smoke-mini PDB could have been deleted on purpose.
		return true, nil
	}

	var allowances bool
	err := rs[0].UnmarshalTo(&allowances)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal allowed disruption: %w", err)
	}

	return allowances, nil
}

func getSmokeMiniImage(values sdkpkg.PatchableValuesCollector) string {
	var (
		registry = values.Get("global.modulesImages.registry.base").String()
		digest   = values.Get("global.modulesImages.digests.upmeter.smokeMini").String()
	)
	return registry + "@" + digest
}

func getSmokeMiniStorageClass(values sdkpkg.PatchableValuesCollector, storageClassSnap []sdkpkg.Snapshot) (string, error) {
	k8s, err := getK8sDefaultStorageClass(storageClassSnap)
	if err != nil {
		return "", err
	}

	d8 := values.Get("global.modules.storageClass").String()
	sm := values.Get("upmeter.smokeMini.storageClass").String()
	return firstNonEmpty(sm, d8, k8s, snapshot.DefaultStorageClass), nil
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
func smokeMiniEnabled(v sdkpkg.PatchableValuesCollector) bool {
	disabled := v.Get("upmeter.smokeMiniDisabled").Bool()
	return !disabled
}
