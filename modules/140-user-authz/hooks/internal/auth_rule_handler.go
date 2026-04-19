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

package internal

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type authorizationRule struct {
	Name      string                 `json:"name"`
	Spec      map[string]interface{} `json:"spec"`
	Namespace string                 `json:"namespace,omitempty"`
}

func ApplyAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	car := &authorizationRule{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Spec:      spec,
	}

	return car, nil
}

func AuthorizationRulesHandler(valuesPath, snapshotKey string) func(_ context.Context, input *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		authorizationRules, err := snapshotsToAuthorizationRulesSlice(input.Snapshots.Get(snapshotKey))
		if err != nil {
			return fmt.Errorf("failed to convert '%s' snapshot to authorization rules: %w", snapshotKey, err)
		}
		input.Values.Set(valuesPath, authorizationRules)
		return nil
	}
}

func snapshotsToAuthorizationRulesSlice(snapshots []pkg.Snapshot) ([]authorizationRule, error) {
	ars := make([]authorizationRule, 0, len(snapshots))
	for ar, err := range sdkobjectpatch.SnapshotIter[authorizationRule](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshot: %w", err)
		}
		ars = append(ars, ar)
	}
	return ars, nil
}
