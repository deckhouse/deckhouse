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

// Package bundle is a module intent rule: Enable when the active bundle of the active edition
// enables the module, a soft Disable otherwise. It never Forbids, so a later intent (Module.spec.enabled)
// overrides it.
package bundle

import (
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
)

const reasonDisabledByBundle = "DisabledByBundle"

// BundleChecker reports whether the active bundle enables a package with this licensing
// (edition.Edition.IsEnabled of the active edition).
type BundleChecker func(license edition.Licensing) bool

// Rule is the bundle rule of one module.
type Rule struct {
	bundleChecker BundleChecker
	licensing     edition.Licensing
}

// NewRule builds the bundle rule of a package's licensing.
func NewRule(bundleChecker BundleChecker, licensing edition.Licensing) *Rule {
	return &Rule{
		bundleChecker: bundleChecker,
		licensing:     licensing,
	}
}

// Decide returns Enable, or Disable with DisabledByBundle.
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
