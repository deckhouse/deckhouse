/*
Copyright 2023 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type dexUserName struct {
	Name string `json:"name"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "users",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "User",
			FilterFunc: applyDexUserNameFilter,
		},
	},
}, getDexUsers)

func getDexUsers(_ context.Context, input *go_hook.HookInput) error {
	snap := input.Snapshots.Get("users")
	users := make([]dexUserName, 0, len(snap))
	for name, err := range sdkobjectpatch.SnapshotIter[string](snap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'users' snapshot: %w", err)
		}
		users = append(users, dexUserName{Name: name})
	}
	input.Values.Set("userAuthn.internal.dexUsersCRDs", users)
	return nil
}

func applyDexUserNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}
