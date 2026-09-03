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

package constant

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
)

const (
	Port       = 5001
	Path       = "/system/deckhouse"
	PathRegexp = "^system/deckhouse"
	Scheme     = "https"
)

var (
	Host         = fmt.Sprintf("registry.d8-system.svc:%d", Port)
	ProxyHost    = fmt.Sprintf("127.0.0.1:%d", Port)
	HostWithPath = fmt.Sprintf("%s/%s", Host, strings.TrimLeft(Path, "/"))

	SupportedCRI         = []CRIType{CRIContainerdV1, CRIContainerdV2}
	ModesRequiringModule = []ModeType{ModeDirect, ModeLocal, ModeProxy}
)

func NodeRegistryAddr(addr string) string {
	return fmt.Sprintf("%s:%d/%s", addr, Port, strings.TrimLeft(Path, "/"))
}

// GenerateProxyEndpoints builds the upstream list of the node load balancer from
// the InternalIP addresses of the control plane nodes.
//
// net.JoinHostPort is used rather than string concatenation so that an IPv6
// address is bracketed: `fd00::1:5001` is ambiguous and would be rejected by
// both the NGINX `server` directive and the endpoint validation.
func GenerateProxyEndpoints(masterNodesIPs []string) []string {
	proxyEndpoints := make([]string, 0, len(masterNodesIPs))
	for _, ip := range masterNodesIPs {
		proxyEndpoints = append(proxyEndpoints, net.JoinHostPort(ip, strconv.Itoa(Port)))
	}
	return proxyEndpoints
}

func IsCRISupported(cri CRIType) bool {
	return slices.Contains(SupportedCRI, cri)
}

func ModuleRequired(mode ModeType) bool {
	return slices.Contains(ModesRequiringModule, mode)
}
