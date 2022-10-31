/*
Copyright 2021 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: manual_daemonset_update ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.1", "internal": {}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", true)

	Context("DS has same generation as pods", func() {
		BeforeEach(func() {
			dspods := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 1
  labels:
    app: controller
    ingress-nginx-manual-update: "true"
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
spec:
  selector:
    matchLabels:
      app: controller
      name: main
  template:
    metadata:
      labels:
        app: controller
        name: main
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
      image: registry.deckhouse.io/image:nginx
      imagePullPolicy: IfNotPresent
  updateStrategy:
    type: OnDelete
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 1
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d89
  namespace: d8-ingress-nginx
revision: 1
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-1
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-2
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-3
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
`
			f.KubeStateSet(dspods)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Pods should not be recreated", func() {
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-1").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-3").Exists()).To(BeTrue())
		})
	})

	Context("DS has new generation", func() {
		BeforeEach(func() {
			dspods := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 2
  labels:
    app: controller
    ingress-nginx-manual-update: "true"
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
spec:
  selector:
    matchLabels:
      app: controller
      name: main
  template:
    metadata:
      labels:
        app: controller
        name: main
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
      image: registry.deckhouse.io/image:nginx
      imagePullPolicy: IfNotPresent
  updateStrategy:
    type: OnDelete
status:
  currentNumberScheduled: 3
  desiredNumberScheduled: 3
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d88
  namespace: d8-ingress-nginx
revision: 1
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d89
  namespace: d8-ingress-nginx
revision: 2
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-1
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-2
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-3
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
`
			f.KubeStateSet(dspods)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("One pod should be deleted", func() {
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-3").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-1").Exists()).To(BeTrue())
		})

		Context("second run", func() {
			BeforeEach(func() {
				delete(f.BindingContextController.Controller.CurrentState, "d8-ingress-nginx/Pod/controller-main-3")
				ff := f.KubeStateSet(`
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 2
  labels:
    app: controller
    ingress-nginx-manual-update: "true"
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
spec:
  selector:
    matchLabels:
      app: controller
      name: main
  template:
    metadata:
      labels:
        app: controller
        name: main
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
      image: registry.deckhouse.io/image:nginx
      imagePullPolicy: IfNotPresent
  updateStrategy:
    type: OnDelete
status:
  currentNumberScheduled: 3
  desiredNumberScheduled: 3
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d89
  namespace: d8-ingress-nginx
revision: 2
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-1
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-2
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "2"
  name: controller-main-4
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
`)
				f.BindingContexts.Set(ff)
				f.RunHook()
			})
			It("Second pod should be deleted", func() {
				Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-1").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-2").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-4").Exists()).To(BeTrue())
			})
		})
	})

	Context("Pods are not ready", func() {
		BeforeEach(func() {
			dspods := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 2
  labels:
    app: controller
    ingress-nginx-manual-update: "true"
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
spec:
  selector:
    matchLabels:
      app: controller
      name: main
  template:
    metadata:
      labels:
        app: controller
        name: main
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
      image: registry.deckhouse.io/image:nginx
      imagePullPolicy: IfNotPresent
  updateStrategy:
    type: OnDelete
status:
  currentNumberScheduled: 3
  desiredNumberScheduled: 3
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d89
  namespace: d8-ingress-nginx
revision: 2
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-1
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "False"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-2
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-3
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
`
			f.KubeStateSet(dspods)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Pods should not be deleted", func() {
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-3").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-2").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-1").Exists()).To(BeTrue())
		})
	})

	Context("DS has wrong count", func() {
		BeforeEach(func() {
			dspods := `
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 2
  labels:
    app: controller
    ingress-nginx-manual-update: "true"
    name: main
  name: controller-main
  namespace: d8-ingress-nginx
spec:
  selector:
    matchLabels:
      app: controller
      name: main
  template:
    metadata:
      labels:
        app: controller
        name: main
    spec:
      containers:
      - args:
        - /nginx-ingress-controller
      image: registry.deckhouse.io/image:nginx
      imagePullPolicy: IfNotPresent
  updateStrategy:
    type: OnDelete
status:
  currentNumberScheduled: 1
  desiredNumberScheduled: 3
---
apiVersion: apps/v1
kind: ControllerRevision
metadata:
  labels:
    app: controller
    name: main
  name: controller-main-f45878d89
  namespace: d8-ingress-nginx
revision: 2
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: controller
    name: main
    pod-template-generation: "1"
  name: controller-main-1
  namespace: d8-ingress-nginx
spec:
  containers:
  - args:
    - /nginx-ingress-controller
    image: registry.deckhouse.io/image:nginx
status:
  conditions:
  - status: "True"
    type: Ready
  phase: Running
`
			f.KubeStateSet(dspods)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Pod should not be deleted", func() {
			Expect(f.KubernetesResource("Pod", "d8-ingress-nginx", "controller-main-1").Exists()).To(BeTrue())
		})
	})
})
