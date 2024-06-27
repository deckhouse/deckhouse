/*
Copyright 2024 Flant JSC

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
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("deckhouse :: hooks :: set_deckhouse_leader_pod ::", func() {
	f := HookExecutionConfigInit("", "")
	const deckhouseTestLeaderPodName = "deckhouse-test-1"
	const deckhouseTestSlavePodName = "deckhouse-test-2"

	_ = os.Setenv("DECKHOUSE_POD", deckhouseTestLeaderPodName)

	Context("One pod in d8-deckhouse - one leader", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(leaderPod)
			f.BindingContexts.Set(st)
			f.GenerateOnStartupContext()

			createFakePod(leaderPod)

			f.RunGoHook()
		})

		It("Shouldn put label to the leader pod", func() {
			Expect(f).To(ExecuteSuccessfully())

			leader := f.KubernetesResource("Pod", d8Namespace, deckhouseTestLeaderPodName)
			Expect(leader.Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels").Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels.leader").Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels.leader").Bool()).To(Equal(true))
		})
	})

	Context("Two pods - Only one is leader", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(slavePod + leaderPod)
			f.BindingContexts.Set(st)
			f.GenerateOnStartupContext()

			createFakePod(leaderPod + slavePod)

			f.RunGoHook()
		})

		It("Should attach leader label to the leader pod", func() {
			Expect(f).To(ExecuteSuccessfully())

			leader := f.KubernetesResource("Pod", d8Namespace, deckhouseTestLeaderPodName)
			Expect(leader.Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels").Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels.leader").Exists()).To(Equal(true))
			Expect(leader.Field("metadata.labels.leader").Bool()).To(Equal(true))

			slave := f.KubernetesResource("Pod", d8Namespace, deckhouseTestSlavePodName)
			Expect(slave.Exists()).To(Equal(true))
			Expect(slave.Field("metadata.labels").Exists()).To(Equal(true))
			Expect(slave.Field("metadata.labels.leader").Exists()).To(Equal(false))
		})
	})
})

const (
	leaderPod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test-1
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:v1.2.3
status:
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: true
`

	slavePod = `
---
apiVersion: v1
kind: Pod
metadata:
  name: deckhouse-test-2
  namespace: d8-system
  labels:
    app: deckhouse
spec:
  containers:
    - name: deckhouse
      image: dev-registry.deckhouse.io/sys/deckhouse-oss:test-me
status:
  containerStatuses:
    - containerID: containerd://9990d3eccb8657d0bfe755672308831b6d0fab7f3aac553487c60bf0f076b2e3
      imageID: dev-registry.deckhouse.io/sys/deckhouse-oss/dev@sha256:d57f01a88e54f863ff5365c989cb4e2654398fa274d46389e0af749090b862d1
      ready: true
`
)

func createFakePod(podConfig string) {
	var pod corev1.Pod
	_ = yaml.Unmarshal([]byte(podConfig), &pod)
	_, _ = dependency.TestDC.MustGetK8sClient().CoreV1().Pods("d8-system").Create(context.TODO(), &pod, metav1.CreateOptions{})
}
