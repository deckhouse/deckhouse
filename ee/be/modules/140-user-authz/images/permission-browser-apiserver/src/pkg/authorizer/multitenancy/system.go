/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package multitenancy

import "regexp"

var systemNamespaces = []string{
	"kube-.*",
	"d8-.*",
	"default",
}

var systemNamespacesRegex []*regexp.Regexp

func init() {
	for _, systemNamespace := range systemNamespaces {
		r, _ := regexp.Compile("^" + systemNamespace + "$")
		systemNamespacesRegex = append(systemNamespacesRegex, r)
	}
}

// isSystemNamespace reports whether ns matches a reserved system pattern.
func isSystemNamespace(namespace string) bool {
	for _, pattern := range systemNamespacesRegex {
		if pattern.MatchString(namespace) {
			return true
		}
	}
	return false
}

func systemNamespaceAllowed(entry *DirectoryEntry, namespace string) bool {
	if entry.AllowAccessToSystemNamespaces {
		return true
	}
	_, ok := entry.AllowedSystemNamespaces[namespace]
	return ok
}
