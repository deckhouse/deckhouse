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

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

const (
	storageFoundationModuleSnapshot       = "storage_foundation_module"
	storageFoundationShipsStorageClassKey = "multitenancyManager.internal.storageFoundationShipsStorageClassGrant"
)

var storageFoundationStorageClassGrantSince = semver.MustParse("1.0.3")

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/160-multitenancy-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         storageFoundationModuleSnapshot,
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "Module",
			NameSelector:                 &types.NameSelector{MatchNames: []string{"storage-foundation"}},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterModuleVersion,
		},
	},
}, setStorageFoundationShipsStorageClassGrant)

func filterModuleVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "properties", "version")
	if err != nil {
		return nil, fmt.Errorf("read properties.version of the Module %s: %w", obj.GetName(), err)
	}
	return version, nil
}

func setStorageFoundationShipsStorageClassGrant(_ context.Context, input *go_hook.HookInput) error {
	ships := false
	if module.IsEnabled("storage-foundation", input) {
		versions, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, storageFoundationModuleSnapshot)
		if err != nil {
			return fmt.Errorf("unmarshal %s snapshot: %w", storageFoundationModuleSnapshot, err)
		}
		if len(versions) > 0 {
			ships = shipsStorageClassGrant(versions[0])
		}
	}
	input.Values.Set(storageFoundationShipsStorageClassKey, ships)
	return nil
}

func shipsStorageClassGrant(version string) bool {
	if version == "" {
		return false
	}
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return true
	}
	release := semver.New(v.Major(), v.Minor(), v.Patch(), "", "")
	return !release.LessThan(storageFoundationStorageClassGrantSince)
}
