/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"
	"strings"
)

const (
	Port   = 5001
	Path   = "/system/deckhouse"
	Scheme = "https"
)

const (
	UnknownVersion = "unknown"
)

var (
	Host      = fmt.Sprintf("embedded-registry.d8-system.svc:%d", Port)
	ProxyHost = fmt.Sprintf("127.0.0.1:%d", Port)

	HostWithPath = fmt.Sprintf("%s/%s", Host, strings.TrimLeft(Path, "/"))
)

func GenerateProxyEndpoints(masterNodesIPs []string) []string {
	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, ip := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, fmt.Sprintf("%s:%d", ip, Port))
	}
	return proxyEndpoints
}
