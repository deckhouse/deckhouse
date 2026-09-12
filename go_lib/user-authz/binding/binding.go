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

// Package binding is the naming contract of the (Cluster)RoleBindings user-authz-controller creates
// for ClusterAuthorizationRules and AuthorizationRules. The controller writes them, the
// authorization webhook and permission-browser recognise them: a binding of a rule is cluster-wide
// by construction, but its scope is the rule's multi-tenancy options, so the consumers must know
// which bindings are rule bindings and which rule each belongs to.
package binding

import "strings"

const (
	// NamePrefix is the common prefix of every binding of every rule: user-authz:<rule>:<postfix>.
	NamePrefix = "user-authz:"

	// LabelHeritage and LabelModule are the Deckhouse module labels every rule binding carries.
	LabelHeritage = "heritage"
	LabelModule   = "module"
	// LabelManagedBy marks the bindings owned by user-authz-controller.
	LabelManagedBy = "user-authz.deckhouse.io/managed-by"

	HeritageValue  = "deckhouse"
	ModuleName     = "user-authz"
	ManagedByValue = "user-authz-controller"
)

// RulePrefix is the name prefix shared by all bindings of the rule.
func RulePrefix(ruleName string) string {
	return NamePrefix + ruleName + ":"
}

// RuleNameOf extracts the rule name from a binding name of the form user-authz:<rule>:<postfix>.
// Rule names are DNS-1123 subdomains, so the second segment is unambiguous.
func RuleNameOf(bindingName string) (string, bool) {
	rest, ok := strings.CutPrefix(bindingName, NamePrefix)
	if !ok {
		return "", false
	}
	name, _, found := strings.Cut(rest, ":")
	if !found || name == "" {
		return "", false
	}
	return name, true
}

// IsRuleBinding reports whether a binding with that name and labels belongs to a rule, whether
// rendered by the Helm chart of the module in earlier releases or by user-authz-controller. Both
// mark their bindings with the module labels, and neither ever rendered a binding outside the
// user-authz: namespace of names.
//
// It deliberately asks only for the prefix and the labels, not for a parseable rule name: consumers
// use it to exclude the bindings of a rule from the grants that hold independently of any rule, and
// there a binding whose name they cannot parse has to be excluded too. Use RuleOf when the rule's
// name itself is needed.
func IsRuleBinding(name string, labels map[string]string) bool {
	if !strings.HasPrefix(name, NamePrefix) {
		return false
	}
	return labels[LabelHeritage] == HeritageValue && labels[LabelModule] == ModuleName
}

// RuleOf returns the rule a binding belongs to: it must carry the module labels and a name of the
// form user-authz:<rule>:<postfix>.
func RuleOf(name string, labels map[string]string) (string, bool) {
	if !IsRuleBinding(name, labels) {
		return "", false
	}
	return RuleNameOf(name)
}
