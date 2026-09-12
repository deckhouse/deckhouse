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

package rules

import "strings"

// IsSystemNamespace reports whether the namespace is one a rule without allowAccessToSystemNamespaces
// must not open: the kube-* and d8-* namespaces, default, and the namespaces of the components that
// preceded Deckhouse (antiopa, loghouse), which some clusters still carry.
func IsSystemNamespace(namespace string) bool {
	switch {
	case strings.HasPrefix(namespace, "kube-"), strings.HasPrefix(namespace, "d8-"):
		return true
	case namespace == "default", namespace == "antiopa", namespace == "loghouse":
		return true
	}
	return false
}
