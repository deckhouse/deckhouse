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

// publishedStaticBlock returns the static block of the published context that carries
// internalNetworkCIDRs, if any. They come from one Secret for every static group, so the first
// entry that has them speaks for the cluster.
func publishedStaticBlock(prior map[string]map[string]interface{}) map[string]interface{} {
	for _, entry := range prior {
		if hasInternalNetworkCIDRs(entry) {
			static, _ := entry["static"].(map[string]interface{})
			return static
		}
	}
	return nil
}

func hasInternalNetworkCIDRs(nodeGroup map[string]interface{}) bool {
	cidrs, found, err := unstructured.NestedSlice(nodeGroup, "static", "internalNetworkCIDRs")
	return found && err == nil && len(cidrs) > 0
}
