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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/scheduler"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

// This hook deletes smoke-mini pod when its PVC is marked for deletion (obtains DeletionTimestamp)
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/upmeter/update_selector_pvc",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:              "pvc",
				ApiVersion:        "v1",
				Kind:              "PersistentVolumeClaim",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPvcTermination,

				ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			},
			{
				Name:              "pods",
				ApiVersion:        "v1",
				Kind:              "Pod",
				NamespaceSelector: namespaceSelector,
				LabelSelector:     labelSelector,
				FilterFunc:        snapshot.NewPodPhase,

				ExecuteHookOnEvents:          pointer.BoolPtr(false),
				ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			},
		},
	},
	deletePodsWithoutPVC,
)

// deletePodsWithoutPVC deletes Pod if its PVC is being deleted, or if it is pending.
func deletePodsWithoutPVC(input *go_hook.HookInput) error {
	if !smokeMiniEnabled(input.Values) {
		return nil
	}

	deleter := scheduler.NewPodDeleter(input.PatchCollector, input.LogEntry)
	pvcs := snapshot.ParsePvcTerminationSlice(input.Snapshots["pvc"])
	pods := snapshot.ParsePodPhaseSlice(input.Snapshots["pods"])

	for x := range indexesForDeletion(pvcs, pods) {
		deleter.Delete(snapshot.Index(x).PodName())
	}
	return nil
}

func indexesForDeletion(pvcs []snapshot.PvcTermination, pods []snapshot.PodPhase) set.Set {
	forDeletion := set.New()
	Xpods := podsIndexSet(pods)
	Xpvcs := pvcsIndexSet(pvcs)

	// Pending pod my fail to schedule due to PVC problem
	for _, pod := range pods {
		if !pod.IsPending {
			continue
		}
		forDeletion.Add(pod.Index().String())
	}

	// Terminating PVC should make Pod delete
	for _, pvc := range pvcs {
		if !pvc.IsTerminating {
			continue
		}
		x := pvc.Index().String()
		if !Xpods.Has(x) {
			// Avoid deleting not existing pod
			continue
		}
		forDeletion.Add(x)
	}

	// Absent (already terminated) PVC should make Pod delete
	for x := range Xpods {
		if !Xpvcs.Has(x) {
			// PVC might have been deleted
			forDeletion.Add(x)
		}
	}

	return forDeletion
}

// smokeMiniEnabled returns true if smoke-mini is not disabled. This function is to avoid reversed
// boolean naming.
func smokeMiniEnabled(v *go_hook.PatchableValues) bool {
	disabled := v.Get("upmeter.smokeMiniDisabled").Bool()
	return !disabled
}

func podsIndexSet(pods []snapshot.PodPhase) set.Set {
	s := set.New()
	for _, pod := range pods {
		s.Add(pod.Index().String())
	}
	return s
}

func pvcsIndexSet(pvcs []snapshot.PvcTermination) set.Set {
	s := set.New()
	for _, pvc := range pvcs {
		s.Add(pvc.Index().String())
	}
	return s
}
