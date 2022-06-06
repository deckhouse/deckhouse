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

var _ = Describe("Kube DNS :: sts discovery ::", func() {
	f := HookExecutionConfigInit(`{"kubeDns": {"internal": {"stsNamespaces": []}}}`, ``)

	Context("With one sts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: nginx
  labels:
    app: nginx
spec:
  ports:
  - port: 80
    name: web
  clusterIP: None
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
  namespace: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  serviceName: "nginx"
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
`, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.stsNamespaces").Array()).To(HaveLen(1))
		})

	})

	Context("With two sts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Service
metadata:
  name: app
  namespace: nginx
  labels:
    app: nginx
spec:
  ports:
  - port: 80
    name: web
  clusterIP: None
  selector:
    app: app
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: app
  namespace: nginx
spec:
  selector:
    matchLabels:
      app: app
  serviceName: "nginx"
  template:
    metadata:
      labels:
        app: app
    spec:
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  namespace: nginx
  labels:
    app: nginx
spec:
  ports:
  - port: 80
    name: web
  clusterIP: None
  selector:
    app: nginx
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
  namespace: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  serviceName: "nginx"
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
---
apiVersion: v1
kind: Service
metadata:
  name: nginx2
  namespace: nginx2
  labels:
    app: nginx
spec:
  ports:
  - port: 80
    name: web
  clusterIP: None
  selector:
    app: nginx2
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
  namespace: nginx2
spec:
  selector:
    matchLabels:
      app: nginx2
  serviceName: "nginx2"
  template:
    metadata:
      labels:
        app: nginx2
    spec:
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
`, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.stsNamespaces").Array()).To(HaveLen(2))
		})

	})

	Context("Without sts", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("kubeDns.internal.stsNamespaces").Array()).To(HaveLen(0))
		})

	})
})
