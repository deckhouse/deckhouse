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

package requirements

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	hasLegacyRBACBindingsKey   = "userAuthzHasLegacyRBACBindings"
	legacyRBACBindingsCountKey = "userAuthz:legacyRBACBindingsCount"
	legacyRBACBindingsListKey  = "userAuthz:legacyRBACBindingsList"
)

func init() {
	checkRequirementFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		// The only supported requirement is "false": the cluster must not contain bindings
		// to the legacy experimental RBAC v2 roles (d8:use:*, d8:manage:*).
		if requirementValue != "false" {
			return true, nil
		}

		countRaw, exists := getter.Get(legacyRBACBindingsCountKey)
		if !exists {
			return true, nil
		}
		count, ok := countRaw.(int)
		if !ok || count == 0 {
			return true, nil
		}

		list := ""
		if listRaw, exists := getter.Get(legacyRBACBindingsListKey); exists {
			list, _ = listRaw.(string)
		}

		return false, fmt.Errorf(
			"cluster contains %d binding(s) to legacy experimental RBAC v2 roles (d8:use:*, d8:manage:*), "+
				"which are renamed to d8:namespace:*, d8:system:* and d8:subsystem:* in this release; "+
				"delete or rebind them before upgrading; bindings: %s",
			count, list,
		)
	}

	requirements.RegisterCheck(hasLegacyRBACBindingsKey, checkRequirementFunc)
}
