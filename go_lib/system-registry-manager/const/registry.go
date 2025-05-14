/*
Copyright 2025 Flant JSC

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
