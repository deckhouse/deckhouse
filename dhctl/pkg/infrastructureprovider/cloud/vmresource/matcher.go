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

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
)

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
	return lookupString(rc.Change.After, rule.FieldEquals.Path) == rule.FieldEquals.Value
}

func lookupString(state map[string]interface{}, dottedPath string) string {
	if state == nil || dottedPath == "" {
		return ""
	}
	var current interface{} = state
	for _, segment := range strings.Split(dottedPath, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = m[segment]
		if !ok {
			return ""
		}
	}
	s, ok := current.(string)
	if !ok {
		return ""
	}
	return s
}
