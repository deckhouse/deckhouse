package namespaces

import "strings"

func IsSystem(nsName string) bool {
	return strings.HasPrefix(nsName, "d8-") ||
		strings.HasPrefix(nsName, "kube-") ||
		strings.HasPrefix(nsName, "upmeter-probe-namespace-")
}
