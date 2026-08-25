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

package bashiblecontext

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// dropsInternalNetworkCIDRs reports whether a derived entry lost the internalNetworkCIDRs its
// published predecessor had. Nodes match their own IP against them (candi/bashible/common-steps/
// all/000_discover_node_ip.sh.tpl:23), so clearing the field silently would rewire every node.
func dropsInternalNetworkCIDRs(prior, current map[string]interface{}) bool {
	return hasInternalNetworkCIDRs(prior) && !hasInternalNetworkCIDRs(current)
}

func hasInternalNetworkCIDRs(nodeGroup map[string]interface{}) bool {
	cidrs, found, err := unstructured.NestedSlice(nodeGroup, "static", "internalNetworkCIDRs")
	return found && err == nil && len(cidrs) > 0
}
