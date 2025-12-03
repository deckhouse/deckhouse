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

var _ = Describe("Modules :: control-plane-manager :: hooks :: lock_main_queue ::", func() {
	const (
		initValuesString       = `{"global": {"clusterIsBootstrapped": true}, "controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = ``
		runningReadyPods       = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    component: control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-1
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    component: control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-b
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
`
		runningNotReadyPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    component: control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-a
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'True'
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager
    component: control-plane-manager
    pod-template-generation: "105"
  name: d8-control-plane-manager-b
  namespace: kube-system
spec:
  nodeName: worker-2
status:
  conditions:
  - type: Ready
    status: 'False'
---
`
		properDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 105
  name: d8-control-plane-manager
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  selector:
    matchLabels:
      app: d8-control-plane-manager
---
`
		justRolledOutDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 106
  name: d8-control-plane-manager
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  selector:
    matchLabels:
      app: d8-control-plane-manager
---
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having all cpm Pods being Ready", func() {
		BeforeEach(func() {
			f.KubeStateSet(runningReadyPods + properDaemonSet)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster having all cpm Pods being Ready but no DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having no cpm Pods but with DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having all cpm Pods being Ready with just rolled new DS", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods + justRolledOutDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster having not Ready cpm Pods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningNotReadyPods + properDaemonSet))
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Cluster having justRolledOutDaemonSet and notUpdatedPods", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(runningReadyPods + justRolledOutDaemonSet))
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})

	})

	Context("Cluster with both main and etcd-only DaemonSets, all pods ready", func() {
		const (
			etcdOnlyReadyPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager-etcd-only
    component: control-plane-manager
    pod-template-generation: "50"
  name: d8-control-plane-manager-etcd-only-a
  namespace: kube-system
spec:
  nodeName: etcd-0
status:
  conditions:
  - type: Ready
    status: 'True'
---
`
			etcdOnlyDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 50
  name: d8-control-plane-manager-etcd-only
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  selector:
    matchLabels:
      app: d8-control-plane-manager-etcd-only
---
`
		)

		BeforeEach(func() {
			f.KubeStateSet(runningReadyPods + properDaemonSet + etcdOnlyReadyPods + etcdOnlyDaemonSet)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with etcd-only DaemonSet but pods not ready", func() {
		const (
			etcdOnlyNotReadyPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager-etcd-only
    component: control-plane-manager
    pod-template-generation: "50"
  name: d8-control-plane-manager-etcd-only-a
  namespace: kube-system
spec:
  nodeName: etcd-0
status:
  conditions:
  - type: Ready
    status: 'False'
---
`
			etcdOnlyDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 50
  name: d8-control-plane-manager-etcd-only
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  selector:
    matchLabels:
      app: d8-control-plane-manager-etcd-only
---
`
		)

		BeforeEach(func() {
			f.KubeStateSet(runningReadyPods + properDaemonSet + etcdOnlyNotReadyPods + etcdOnlyDaemonSet)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("Cluster with etcd-only DaemonSet rolled out but pods not updated", func() {
		const (
			etcdOnlyOldPods = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: d8-control-plane-manager-etcd-only
    component: control-plane-manager
    pod-template-generation: "49"
  name: d8-control-plane-manager-etcd-only-a
  namespace: kube-system
spec:
  nodeName: etcd-0
status:
  conditions:
  - type: Ready
    status: 'True'
---
`
			etcdOnlyDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  generation: 50
  name: d8-control-plane-manager-etcd-only
  namespace: kube-system
  labels:
    app: d8-control-plane-manager
spec:
  selector:
    matchLabels:
      app: d8-control-plane-manager-etcd-only
---
`
		)

		BeforeEach(func() {
			f.KubeStateSet(runningReadyPods + properDaemonSet + etcdOnlyOldPods + etcdOnlyDaemonSet)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("Should exit with error", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

})
