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

// Package requirements gates a Deckhouse update on how the default Pod Security Standards policy
// is chosen in the cluster.
//
// Up to DKP 1.77 a hook derives the default policy from the version the cluster was bootstrapped
// with when the ModuleConfig does not set it: Privileged for a version lower than v1.55 or an
// unknown version, Baseline otherwise. DKP 1.78 removes the hook and makes Baseline the unconditional
// default, so a cluster on the implicit Privileged policy would switch to Baseline on update and
// start denying pods that do not comply with it.
//
// A requirement is evaluated by the version already INSTALLED. The check therefore has to ship in
// 1.77, where the hook still computes the policy and records whether it is an implicit Privileged
// one under ImplicitPrivilegedKey. The release that removes the hook declares the requirement in
// release.yaml. From that release on nothing records the value, and the check passes: the default
// no longer depends on the bootstrap version, so there is nothing left to protect.
package requirements

import (
	"fmt"
	"strconv"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	// ImplicitPrivilegedKey is both the release requirement key and the key of the value the
	// module records. The value is true when the ModuleConfig does not set
	// `podSecurityStandards.defaultPolicy` and the module derives Privileged for it.
	//
	// The release that makes Baseline the unconditional default requires it to be "false".
	ImplicitPrivilegedKey = "pssDefaultPolicyImplicitPrivileged"
)

func init() {
	requirements.RegisterCheck(ImplicitPrivilegedKey, check)
}

// check compares what a release allows against how the cluster chooses its default policy.
//
// An absent value passes: either the module has not run yet, or the installed version no longer
// derives the default policy and has nothing to record.
func check(required string, getter requirements.ValueGetter) (bool, error) {
	allowed, err := strconv.ParseBool(required)
	if err != nil {
		return false, fmt.Errorf("parse requirement value %q: %w", required, err)
	}

	raw, exists := getter.Get(ImplicitPrivilegedKey)
	if !exists {
		return true, nil
	}

	implicitPrivileged, ok := toBool(raw)
	if !ok || !implicitPrivileged || allowed {
		return true, nil
	}

	return false, fmt.Errorf(
		"the default Pod Security Standards policy is not set in the admission-policy-engine ModuleConfig, " +
			"and the module applies Privileged because the cluster was bootstrapped with a Deckhouse version " +
			"lower than v1.55 or the bootstrap version is unknown; this release makes Baseline the default policy " +
			"for all non-system namespaces; set `settings.podSecurityStandards.defaultPolicy` in the " +
			"admission-policy-engine ModuleConfig explicitly: Privileged to keep the current behavior, " +
			"or Baseline or Restricted after making sure that the workloads comply with the chosen policy")
}

// toBool tolerates both the in-memory (bool) and a serialized (string) representation of the value.
func toBool(raw any) (bool, bool) {
	switch v := raw.(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(v)
		return b, err == nil
	default:
		return false, false
	}
}
