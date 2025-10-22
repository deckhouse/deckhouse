// Copyright 2021 Flant JSC
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

/*

User-stories:
1. If there is a Deployment kube-system/cluster-autoscaler in cluster, it must not have section `resources.limits` because extended-monitoring will alert at throttling.

*/

package hooks

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	initValuesString       = `{}`
	initConfigValuesString = `{}`
)

const (
	stateLimitsAreSet = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  replicas: 2
  template:
    spec:
      containers:
      - image: good-image
        resources:
          requests:
            cpu: 100m
            memory: 300Mi
          limits:
            cpu: 333m
            memory: 333Mi`

	stateLimitsAreUnset = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  replicas: 2
  template:
    spec:
      containers:
      - image: good-image
        resources:
          requests:
            cpu: 100m
            memory: 300Mi`
)

var _ = Describe("Global hooks :: resources/cluster_autoscaler ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	assertKeepsRequestsInContainer := func(f *HookExecutionConfig) {
		d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")
		Expect(d.Field("spec.template.spec.containers.0.resources").Exists()).To(BeTrue())
		Expect(d.Field("spec.template.spec.containers.0.resources.requests").Exists()).To(BeTrue())
	}

	assertKeepsContainerAndDeployment := func(f *HookExecutionConfig) {
		d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")

		Expect(d.Field("spec.replicas").Int()).To(Equal(int64(2)))

		containers := d.Field("spec.template.spec.containers").Array()
		Expect(containers).To(HaveLen(1))

		Expect(d.Field("spec.template.spec.containers.0.image").String()).To(Equal("good-image"))
	}

	Context("There is no Deployment kube-system/cluster-autoscaler in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("Someone created Deployment kube-system/cluster-autoscaler with `spec.template.spec.containers.0.resources.limits`", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet))
				f.RunHook()
			})

			It("section `limits` must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")
				Expect(d.Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
			})

			It("keeps 'requests' in container", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertKeepsRequestsInContainer(f)
			})

			It("keeps another container and deployment fields", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertKeepsContainerAndDeployment(f)
			})
		})
	})

	Context("There is Deployment kube-system/cluster-autoscaler in cluster with section `spec.template.spec.containers.0.resources.limits`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet))
			f.RunHook()
		})

		It("section `limits` must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")
			Expect(d.Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
		})

		It("keeps 'requests' in container", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertKeepsRequestsInContainer(f)
		})

		It("keeps another container and deployment fields", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertKeepsContainerAndDeployment(f)
		})
	})

	Context("There is Deployment kube-system/cluster-autoscaler in cluster without section `spec.template.spec.containers.0.resources.limits`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreUnset))
			f.RunHook()
		})

		It("section `limits` must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())

			d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")
			Expect(d.Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
		})

		It("keeps 'requests' in container", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertKeepsRequestsInContainer(f)
		})

		It("keeps another container and deployment fields", func() {
			Expect(f).To(ExecuteSuccessfully())

			assertKeepsContainerAndDeployment(f)
		})

		Context("Someone modified Deployment kube-system/cluster-autoscaler by adding section `spec.template.spec.containers.0.resources.limits`", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateLimitsAreSet))
				f.RunHook()
			})

			It("section `limits` must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())

				d := f.KubernetesResource("Deployment", "kube-system", "cluster-autoscaler")
				Expect(d.Field("spec.template.spec.containers.0.resources.limits").Exists()).To(BeFalse())
			})

			It("keeps 'requests' in container", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertKeepsRequestsInContainer(f)
			})

			It("keeps another container and deployment fields", func() {
				Expect(f).To(ExecuteSuccessfully())

				assertKeepsContainerAndDeployment(f)
			})
		})
	})
})
