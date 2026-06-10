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

package vmresource

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

// Match reports whether rc matches rule.
//
// Three outcomes are folded into the bool return for callers that only need
// a yes/no answer (IsVMChange). lookupFieldString below preserves the
// distinction internally — a missing field and a present-but-non-string
// field both fail the match, but a non-string field also returns an error
// that the caller can surface separately when needed.
func Match(rc plan.ResourceChange, rule *Rule) bool {
	if rule == nil {
		return false
	}
	if rc.Type != rule.Type {
		return false
	}
	if rule.FieldEquals == nil {
		return true
	}
	value, _, err := lookupFieldString(rc.Change.After, rule.FieldEquals.Path)
	if err != nil {
		return false
	}
	return value == rule.FieldEquals.Value
}

// lookupFieldString resolves a dotted path against an unstructured object.
// It mirrors unstructured.NestedString semantics so callers can distinguish:
//
//   - ("value", true, nil)  — found, string
//   - ("", false, nil)       — not found at any segment along the path
//   - ("", false, err)       — found, but the leaf is not a string (typed
//     mismatch — most likely an authoring bug in plan_rules.yml)
func lookupFieldString(state map[string]interface{}, dottedPath string) (string, bool, error) {
	if state == nil || dottedPath == "" {
		return "", false, nil
	}
	segments := strings.Split(dottedPath, ".")
	return unstructured.NestedString(state, segments...)
}
