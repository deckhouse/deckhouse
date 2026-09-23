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
	"time"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	// idTokenTTLRequirementKey is the release requirement key. A release.yaml sets it to the
	// idTokenTTL the release refuses (a duration, e.g. "6h"): the release is not deployed while the
	// user-authn ModuleConfig sets idTokenTTL to that value or more.
	//
	// The check runs in the version already INSTALLED, before it switches to the release, so a
	// release that makes the value invalid for the ModuleConfig OpenAPI schema does not reach a
	// cluster that would fail to start with it: addon-operator validates stored ModuleConfigs at
	// startup and does not start on a violation.
	idTokenTTLRequirementKey = "userAuthnIDTokenTTLBelow"

	// idTokenTTLValueKey mirrors hooks.IDTokenTTLValueKey (the packages must not import each
	// other's internals; the contract is locked by tests). The hook stores the idTokenTTL string of
	// the user-authn ModuleConfig, "" when the field is unset.
	idTokenTTLValueKey = "userAuthn:idTokenTTL"
)

func init() {
	requirements.RegisterCheck(idTokenTTLRequirementKey, checkIDTokenTTL)
}

func checkIDTokenTTL(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	limit, err := time.ParseDuration(requirementValue)
	if err != nil {
		return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
	}

	raw, exists := getter.Get(idTokenTTLValueKey)
	if !exists {
		// The hook has not published a value (the module is disabled or has not synced yet).
		return true, nil
	}

	ttl, _ := raw.(string)
	if ttl == "" {
		return true, nil
	}

	d, err := time.ParseDuration(ttl)
	if err != nil {
		// The OpenAPI pattern admits only h/m/s durations; a value that does not parse is not
		// this check's to judge.
		return true, nil
	}

	if d < limit {
		return true, nil
	}

	return false, fmt.Errorf(
		"the user-authn ModuleConfig sets idTokenTTL to %s; the new release requires it to be less than %s "+
			"(Dex rotates its signing keys every 6 hours) and would not start otherwise: "+
			"lower spec.settings.idTokenTTL of ModuleConfig user-authn, for example to 1h",
		ttl, requirementValue)
}
