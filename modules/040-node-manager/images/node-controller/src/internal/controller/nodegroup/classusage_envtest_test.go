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

package nodegroup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

func createTestInstanceClass(name string) *unstructured.Unstructured {
	class := instanceClass(testClassKind, name, nil)
	Expect(k8sClient.Create(suiteCtx, class)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, class))).To(Succeed())
	})
	return class
}

func createTestClassRegistration() {
	secret := registrationSecret(testenv.UniqueName("cloud-provider-d8test"), map[string][]byte{
		nodecommon.InstanceClassKindKey:       []byte(testClassKind),
		nodecommon.InstanceClassAPIVersionKey: []byte("v1"),
	})
	Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, secret))).To(Succeed())
	})
}

func classConsumerNodeGroup(name, className string) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				MinPerZone:     1,
				MaxPerZone:     1,
				ClassReference: v1.ClassReference{Kind: testClassKind, Name: className},
			},
		},
	}
}

// instanceClassConsumers returns status.nodeGroupConsumers of the class and asserts the field is
// there at all: an absent field and an empty list are different answers to the deletion webhook.
func instanceClassConsumers(g Gomega, name string) []string {
	class := &unstructured.Unstructured{}
	class.SetGroupVersionKind(classGVK(testClassKind))
	g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, class)).To(Succeed())

	consumers, found, err := unstructured.NestedStringSlice(class.Object, "status", "nodeGroupConsumers")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue(), "status.nodeGroupConsumers of %s must be published, not absent", name)
	return consumers
}

var _ = Describe("InstanceClass consumers", func() {
	BeforeEach(func() {
		createClusterKubernetesConfigMap(defaultDesiredKubernetesVersion)
	})

	It("publishes the consuming NodeGroup and clears it once the group is gone", func() {
		createTestClassRegistration()
		usedName := testenv.UniqueName("used-class")
		spareName := testenv.UniqueName("spare-class")
		createTestInstanceClass(usedName)
		createTestInstanceClass(spareName)

		ngName := uniqueNG("class-consumer")
		ng := createNodeGroup(classConsumerNodeGroup(ngName, usedName))

		By("publishing the consumer on the referenced class and an empty list on the spare one")
		Eventually(func(g Gomega) {
			g.Expect(instanceClassConsumers(g, usedName)).To(Equal([]string{ngName}))
			g.Expect(instanceClassConsumers(g, spareName)).To(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("clearing the consumer after the NodeGroup is deleted")
		Expect(k8sClient.Delete(suiteCtx, ng)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(instanceClassConsumers(g, usedName)).To(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})
})
