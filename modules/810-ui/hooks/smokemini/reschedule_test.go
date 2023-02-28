/*
Copyright 2023 Flant JSC

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

package smokemini

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: upmeter :: hooks :: smoke_mini_rescheduler ::", func() {
	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{
"upmeter":{
  "internal":{
    "smokeMini":{
      "sts":{"a":{},"b":{},"c":{},"d":{},"e":{}}
    }
  }
}}`, `{}`)

		DescribeTable("version change",
			func(state string, quantity int) {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(state, quantity))
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			},
			Entry("One node, no pods", `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
`, 1),
			Entry("One node and a pod on it", `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  annotations:
    node: node-a-1
    zone: nova
  labels:
    app: smoke-mini
    module: upmeter
  name: smoke-mini-a
  namespace: d8-upmeter
spec:
  IndexSelector:
    matchLabels:
      smoke-mini: a
  serviceName: smoke-mini-a
  template:
    metadata:
      labels:
        app: smoke-mini
        smoke-mini: a
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/hostname
                  operator: In
                  values:
                  - node-a-1
      containers:
      - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
        name: smoke-mini
status:
  collisionCount: 0
  currentReplicas: 1
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1
---
apiVersion: v1
kind: Pod
metadata:
  name: smoke-mini-a-0
  namespace: d8-upmeter
spec:
  nodeName: node-a-1
  containers:
  - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
    name: smoke-mini
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/hostname
              operator: In
              values:
              - node-a-1
`, 3),
			Entry("Two nodes and a pod", `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-2
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-2
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  annotations:
    node: node-a-1
    zone: nova
  labels:
    app: smoke-mini
    module: upmeter
  name: smoke-mini-a
  namespace: d8-upmeter
spec:
  IndexSelector:
    matchLabels:
      smoke-mini: a
  serviceName: smoke-mini-a
  template:
    metadata:
      labels:
        app: smoke-mini
        smoke-mini: a
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/hostname
                  operator: In
                  values:
                  - node-a-1
      containers:
      - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
        name: smoke-mini
status:
  collisionCount: 0
  currentReplicas: 1
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1
---
apiVersion: v1
kind: Pod
metadata:
  name: smoke-mini-a-0
  namespace: d8-upmeter
spec:
  nodeName: node-a-1
  containers:
  - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
    name: smoke-mini
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
            - key: kubernetes.io/hostname
              operator: In
              values:
              - node-a-1
`, 4),
			Entry("Unscheduled pod", `
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  annotations:
    node: node-a-1
    zone: nova
  labels:
    app: smoke-mini
    module: upmeter
  name: smoke-mini-a
  namespace: d8-upmeter
spec:
  IndexSelector:
    matchLabels:
      smoke-mini: a
  serviceName: smoke-mini-a
  template:
    metadata:
      labels:
        app: smoke-mini
        smoke-mini: a
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: kubernetes.io/hostname
                  operator: In
                  values:
                  - node-a-1
      containers:
      - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
        name: smoke-mini
status:
  collisionCount: 0
  currentReplicas: 1
  readyReplicas: 1
  replicas: 1
  updatedReplicas: 1
---
apiVersion: v1
kind: Pod
metadata:
  name: smoke-mini-a-0
  namespace: d8-upmeter
spec:
  nodeName: ""
  containers:
    - image: registry.deckhouse.io/deckhouse/ce/upmeter/smoke-mini:whatever
      name: smoke-mini
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - weight: 1
        nodeSelectorTerms:
          matchExpressions:
          - key: kubernetes.io/hostname
            operator: In
            values:
            - node-a-1
`, 3))
	})

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"upmeter":{"smokeMiniDisabled": true}}`, `{}`)

		It("Should execute successfully", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Node
metadata:
  labels:
    kubernetes.io/hostname: node-a-1
    failure-domain.beta.kubernetes.io/zone: nova
  name: node-a-1
status:
  conditions:
  - status: "True"
    type: Ready
`, 1))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
		})
	})
})

func Test_firstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "none",
			want: "",
		},
		{
			name: "one empty",
			args: []string{""},
			want: "",
		},
		{
			name: "multiple starting with empty",
			args: []string{"", "a", "b"},
			want: "a",
		},
		{
			name: "multiple starting with non-empty",
			args: []string{"x", "", "a"},
			want: "x",
		},
		{
			name: "one filled among empty",
			args: []string{"", "", "z", "", ""},
			want: "z",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.args...); got != tt.want {
				t.Errorf("firstNonEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
