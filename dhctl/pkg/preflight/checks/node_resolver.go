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

package checks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// nodeResolverFix is what to do about a node that cannot resolve hostname.
//
// The general advice is about the node's own resolver. A cloud node, though, is one dhctl has just
// created and gets its resolver from the cloud: a resolv.conf edited by hand is lost with the
// node. Where the provider configuration is what decides the resolver, the advice names that
// field instead.
func nodeResolverFix(meta *config.MetaConfig, hostname string) string {
	if fix := dynamixResolverFix(meta, hostname); fix != "" {
		return fix
	}

	return fmt.Sprintf("check /etc/resolv.conf on the node and give it a resolver that knows %s, "+
		"or add the address to /etc/hosts on the node", hostname)
}

// dynamixResolverFix covers the Standard layout, where a node has only the external network. The
// network hands out DNS servers only if the platform administrator set them on it, and the
// nameservers of DynamixClusterConfiguration are what the module writes into the node's
// cloud-init otherwise. On StandardWithInternalNetwork the ViNS hands the nodes those same
// servers, so there is nothing to add to the general advice.
func dynamixResolverFix(meta *config.MetaConfig, hostname string) string {
	if meta == nil || meta.ProviderName != "dynamix" {
		return ""
	}

	var layout string
	if err := json.Unmarshal(meta.ProviderClusterConfig["layout"], &layout); err != nil || layout != "Standard" {
		return ""
	}

	// A field that cannot be read is not an empty field: telling the user that
	// nameservers is empty when it is set, only in a shape this cannot parse, sends them
	// looking in the wrong place. The general advice is the honest answer then.
	var nameservers []string
	if raw, set := meta.ProviderClusterConfig["nameservers"]; set {
		if err := json.Unmarshal(raw, &nameservers); err != nil {
			return ""
		}
	}

	if len(nameservers) == 0 {
		return fmt.Sprintf("on layout Standard the nodes get DNS servers from the external network only, and "+
			"DynamixClusterConfiguration.nameservers is empty. Set it to DNS servers the nodes can reach and that "+
			"resolve %s, or have the platform administrator set DNS servers on the external network", hostname)
	}

	return fmt.Sprintf("the nodes use DynamixClusterConfiguration.nameservers (%s): check that they resolve %s "+
		"and that the nodes reach them, or correct DynamixClusterConfiguration.nameservers",
		strings.Join(nameservers, ", "), hostname)
}
