// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/iancoleman/strcase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_roles",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyDeckhouseNodesFilter,
		},
	},
}, countDeckhouseNodes)

func applyDeckhouseNodesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var roles []string
	for l := range obj.GetLabels() {
		if strings.HasPrefix(l, "node-role.deckhouse.io/") {
			role := strings.Split(l, "/")[1]
			roles = append(roles, role)
		}
	}

	// small optimization
	// checksum for sorted array always same (map is unordered)
	// and we will not run hook if roles order changed in the map
	sort.Strings(roles)

	return roles, nil
}

func countDeckhouseNodes(_ context.Context, input *go_hook.HookInput) error {
	nodeRoles, err := sdkobjectpatch.UnmarshalToStruct[[]string](input.Snapshots, "node_roles")
	if err != nil {
		return fmt.Errorf("failed to unmarshal node_roles snapshot: %w", err)
	}

	nodesCountByRole := make(map[string]int)
	for _, rolesForNode := range nodeRoles {
		for _, role := range rolesForNode {
			if role != "" {
				roleCamel := strcase.ToLowerCamel(role)
				nodesCountByRole[roleCamel]++
			}
		}
	}

	input.Values.Set("global.discovery.d8SpecificNodeCountByRole", nodesCountByRole)

	return nil
}
