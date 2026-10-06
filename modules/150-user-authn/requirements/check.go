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
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks"
)

const (
	// idTokenTTLRequirementKey is the release requirement key. A release.yaml sets it to a duration
	// (e.g. "6h"), and the release is not deployed while the user-authn ModuleConfig sets idTokenTTL
	// to that value or more.
	//
	// The check runs in the version already INSTALLED, before it switches to the release. It holds
	// the update until the value is lowered, so the operator chooses when that happens. Without it
	// the update goes ahead: the next version of the module settings accepts only values below 6h,
	// and converting the ModuleConfig to it replaces a value of 6h or more with 5h59m, which
	// shortens DexAuthenticator sessions and can get ID tokens already issued rejected before they
	// expire (see the D8UserAuthnIDTokenTTLTooLong alert).
	idTokenTTLRequirementKey = "userAuthnIDTokenTTLBelow"
)

func init() {
	requirements.RegisterCheck(idTokenTTLRequirementKey, checkIDTokenTTL)
}

func checkIDTokenTTL(requirementValue string, getter requirements.ValueGetter) (bool, error) {
	limit, err := time.ParseDuration(requirementValue)
	if err != nil {
		return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
	}

	// The hook stores the idTokenTTL string of the user-authn ModuleConfig, "" when unset.
	raw, exists := getter.Get(hooks.IDTokenTTLValueKey)
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
		"the user-authn ModuleConfig sets idTokenTTL to %s; the update waits until it is less than %s "+
			"(Dex rotates its signing keys every 6 hours): "+
			"lower spec.settings.idTokenTTL of ModuleConfig user-authn, for example to 1h; "+
			"the D8UserAuthnIDTokenTTLTooLong alert describes what lowering it changes",
		ttl, requirementValue)
}
