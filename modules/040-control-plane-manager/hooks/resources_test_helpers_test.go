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

package hooks

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/gomega"
)

func autotuneStateYAML(state autotuneState) string {
	raw, err := json.Marshal(state)
	Expect(err).ToNot(HaveOccurred())
	// Embed JSON as a single-line string value for the ConfigMap.
	escaped, err := json.Marshal(string(raw))
	Expect(err).ToNot(HaveOccurred())
	return fmt.Sprintf(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: kube-system
data:
  state: %s
`, autotuneStateCMName, string(escaped))
}

func masterNodeYAML() string {
	return generateMasterNodesConfig([]masterNode{{
		cpu:    "8",
		memory: "16Gi",
		capCPU: "8",
		capMem: "16Gi",
	}})
}

// setNearFallbackUsage fills usage for all four components near the %-split of
// combined=2000m / 4Gi so the initial-snapshot gate can pass. Override individual
// entries afterward when a test needs an out-of-band recommendation.
func setNearFallbackUsage(usage map[string]map[resourceKind]float64) {
	usage[componentKubeApiserver] = map[resourceKind]float64{
		resourceCPU:    0.66,
		resourceMemory: 1417339207, // ~33% of 4Gi
	}
	usage[componentEtcd] = map[resourceKind]float64{
		resourceCPU:    0.70,
		resourceMemory: 1503238553, // ~35% of 4Gi
	}
	usage[componentKubeControllerManager] = map[resourceKind]float64{
		resourceCPU:    0.40,
		resourceMemory: 858993459, // ~20% of 4Gi
	}
	usage[componentKubeScheduler] = map[resourceKind]float64{
		resourceCPU:    0.20,
		resourceMemory: 429496729, // ~10% of 4Gi
	}
}
