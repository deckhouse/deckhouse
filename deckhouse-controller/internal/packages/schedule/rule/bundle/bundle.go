// Copyright 2026 Flant JSC
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

// Package bundle provides a module floor rule: it enables a module when the
// cluster's active bundle is one that enables it in the active edition, and
// soft-disables it otherwise. It is intent, not a gate — the decision is always
// overridable by a higher-precedence rule (e.g. the user's ModuleConfig), so it
// never Forbids.
package bundle

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

// reasonDisabledByBundle is recorded when the active bundle does not enable the
// module. Matches the Kubernetes condition reason pattern
// ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$.
const reasonDisabledByBundle = "DisabledByBundle"

// BundleChecker reports whether the active bundle enables the package given its
// licensing. It is the edition's bundle check (edition.Edition.IsEnabled) bound
// to the active edition, so the rule resolves the decision live against whatever
// edition/bundle the cluster runs.
type BundleChecker func(license edition.Licensing) bool

// Rule is a module floor keyed on bundle membership. Which bundles enable the
// module depends on the active edition, so the rule defers to a BundleChecker
// (the active edition's IsEnabled) over the package's licensing: it Enables the
// module when the checker passes and soft-Disables it otherwise.
type Rule struct {
	bundleChecker BundleChecker
	licensing     edition.Licensing
}

// NewRule builds a bundle rule from the active edition's bundle check and the
// package's licensing.
func NewRule(bundleChecker BundleChecker, licensing edition.Licensing) *Rule {
	return &Rule{
		bundleChecker: bundleChecker,
		licensing:     licensing,
	}
}

// Decide enables the module when the active bundle enables it in the active
// edition; otherwise it soft-disables it with the DisabledByBundle reason. Never
// Forbids — bundle membership is intent, overridable by a higher-precedence rule.
func (r *Rule) Decide() rule.Decision {
	if r.bundleChecker(r.licensing) {
		return rule.Decision{Kind: rule.Enable}
	}

	return rule.Decision{
		Kind:    rule.Disable,
		Reason:  reasonDisabledByBundle,
		Message: "module is not enabled in the active bundle",
	}
}
