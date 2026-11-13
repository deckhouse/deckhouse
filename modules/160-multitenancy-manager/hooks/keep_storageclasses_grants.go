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
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// The previous release of this module rendered the StorageClass grant objects without the keep
// policy, so on the first run of this release with storage-foundation enabled it would prune them if
// it owned them at that moment. The keep policy set on the live objects prevents that.
//
// TODO: remove in the first minor release after the next LTS release following the release that
// introduced this hook. LTS clusters jump up to 10 minors
// (deckhouse-controller/pkg/releaseupdater/task_calculator.go, maxMinorVersionDiffForLTS), so that
// LTS must still run this hook. Also make sure no update constraint in release metadata skips the
// release that introduced it.

const (
	storageClassesDefinitionsSnapshot = "storageclasses_definitions"
	storageClassesReferencesSnapshot  = "storageclasses_references"
)

var (
	storageClassesDefinitionNames = []string{"storageclasses"}
	storageClassesReferenceNames  = []string{"storageclasses-pvc"}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/160-multitenancy-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 2},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         storageClassesDefinitionsSnapshot,
			ApiVersion:                   "multitenancy.deckhouse.io/v1alpha1",
			Kind:                         "GrantableClusterResourceDefinition",
			NameSelector:                 &types.NameSelector{MatchNames: storageClassesDefinitionNames},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterObjectName,
		},
		{
			Name:                         storageClassesReferencesSnapshot,
			ApiVersion:                   "multitenancy.deckhouse.io/v1alpha1",
			Kind:                         "GrantableClusterResourceReference",
			NameSelector:                 &types.NameSelector{MatchNames: storageClassesReferenceNames},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterObjectName,
		},
	},
}, keepStorageClassesGrants)

func keepStorageClassesGrants(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get(storageFoundationShipsStorageClassKey).Bool() {
		return nil
	}
	patch := map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{resourcePolicyAnnotation: resourcePolicyKeep}},
	}
	for _, s := range []struct {
		snapshot, kind string
		names          []string
	}{
		{storageClassesDefinitionsSnapshot, "GrantableClusterResourceDefinition", storageClassesDefinitionNames},
		{storageClassesReferencesSnapshot, "GrantableClusterResourceReference", storageClassesReferenceNames},
	} {
		names, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, s.snapshot)
		if err != nil {
			return fmt.Errorf("unmarshal %s snapshot: %w", s.snapshot, err)
		}
		for _, name := range names {
			if !slices.Contains(s.names, name) {
				continue
			}
			input.PatchCollector.PatchWithMerge(patch, "multitenancy.deckhouse.io/v1alpha1", s.kind, "", name,
				object_patch.WithIgnoreMissingObject())
		}
	}
	return nil
}
