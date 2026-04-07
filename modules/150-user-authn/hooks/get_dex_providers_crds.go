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

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type DexProvider map[string]interface{}

func applyDexProviderFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex provider: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex provider has no spec field")
	}

	spec["id"] = obj.GetName()
	return DexProvider(spec), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "providers",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "DexProvider",
			FilterFunc: applyDexProviderFilter,
		},
	},
}, getDexProviders)

func getDexProviders(_ context.Context, input *go_hook.HookInput) error {
	providers, err := sdkobjectpatch.UnmarshalToStruct[map[string]interface{}](input.Snapshots, "providers")

	if err != nil {
		input.Values.Set("userAuthn.internal.providers", []interface{}{})
		return nil
	}

	// Filter out providers with spec.enabled == false. Absence of the field is treated as enabled=true.
	filtered := make([]map[string]interface{}, 0, len(providers))
	for _, p := range providers {
		// p corresponds to the .spec map with injected "id"
		if enabledRaw, ok := p["enabled"]; ok {
			if enabledBool, ok := enabledRaw.(bool); ok && !enabledBool {
				// skip disabled provider
				continue
			}
		}

		// Sanitize LDAP filter values to remove trailing whitespace/newlines
		// that may appear when users use YAML literal block scalars (|) in CRDs.
		sanitizeLDAPFilters(p)

		// Keep 'enabled' field in internal values for transparency and debugging
		filtered = append(filtered, p)
	}

	input.Values.Set("userAuthn.internal.providers", filtered)
	return nil
}

// sanitizeLDAPFilters trims whitespace from LDAP filter fields in userSearch and groupSearch.
// This prevents issues when users specify filters using YAML literal block scalars (|),
// which add a trailing newline that breaks LDAP search queries.
func sanitizeLDAPFilters(provider map[string]interface{}) {
	ldapRaw, ok := provider["ldap"]
	if !ok {
		return
	}
	ldapMap, ok := ldapRaw.(map[string]interface{})
	if !ok {
		return
	}
	trimFilterInSearchSection(ldapMap, "userSearch")
	trimFilterInSearchSection(ldapMap, "groupSearch")
}

// trimFilterInSearchSection trims whitespace from the "filter" field
// within a given LDAP search section (userSearch or groupSearch).
func trimFilterInSearchSection(ldapMap map[string]interface{}, section string) {
	sectionRaw, ok := ldapMap[section]
	if !ok {
		return
	}
	sectionMap, ok := sectionRaw.(map[string]interface{})
	if !ok {
		return
	}
	if filter, ok := sectionMap["filter"].(string); ok {
		sectionMap["filter"] = strings.TrimSpace(filter)
	}
}
