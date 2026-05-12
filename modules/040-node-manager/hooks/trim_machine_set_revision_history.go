/*
Copyright 2026 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	mcmMachineSetAPIVersion               = "machine.sapcloud.io/v1alpha1"
	mcmMachineSetRevisionHistoryKey       = "deployment.kubernetes.io/revision-history"
	mcmMachineSetRevisionHistoryMaxLength = 16
)

type mcmMachineSetRevisionHistory struct {
	Name            string
	Namespace       string
	RevisionHistory string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/trim_machine_set_revision_history",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "mcm_machinesets",
			ApiVersion:             mcmMachineSetAPIVersion,
			Kind:                   "MachineSet",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: mcmMachineSetRevisionHistoryFilter,
		},
	},
}, handleTrimMachineSetRevisionHistory)

func mcmMachineSetRevisionHistoryFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()

	return mcmMachineSetRevisionHistory{
		Name:            obj.GetName(),
		Namespace:       obj.GetNamespace(),
		RevisionHistory: annotations[mcmMachineSetRevisionHistoryKey],
	}, nil
}

func handleTrimMachineSetRevisionHistory(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("mcm_machinesets")

	for ms, err := range sdkobjectpatch.SnapshotIter[mcmMachineSetRevisionHistory](snaps) {
		if err != nil {
			return fmt.Errorf("iterate MachineSet snapshots: %w", err)
		}

		if len(ms.RevisionHistory) <= mcmMachineSetRevisionHistoryMaxLength {
			continue
		}

		revisionHistory := trimMachineSetRevisionHistory(ms.RevisionHistory)
		if revisionHistory == ms.RevisionHistory {
			continue
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					mcmMachineSetRevisionHistoryKey: revisionHistory,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, mcmMachineSetAPIVersion, "MachineSet", ms.Namespace, ms.Name)
	}

	return nil
}

func trimMachineSetRevisionHistory(revisionHistory string) string {
	firstRevision, _, _ := strings.Cut(revisionHistory, ",")

	return firstRevision
}
