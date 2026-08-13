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

package capi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The version decides the value of every field the instance-class checksum hashes, and that
// checksum names the infrastructure MachineTemplate. The name is immutable, so a checksum that
// moves renames the template and CAPI rolls every machine in the NodeGroup.
//
// Two versions of the same object cannot be made to differ inside envtest (that needs the
// provider's conversion webhook), so these specs pin the decision instead of the divergence: the
// published version is the one used, and a version that cannot serve the read stops the rendering
// rather than being quietly swapped for one that can.
var _ = Describe("InstanceClass API version pinning", func() {
	const icName = "version-pinning"

	newInstanceClass := func() *unstructured.Unstructured {
		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName(icName)
		Expect(unstructured.SetNestedField(ic.Object, "test", "spec", "vmClassName")).To(Succeed())
		return ic
	}

	newNodeGroup := func(name string) *deckhousev1.NodeGroup {
		ng := &deckhousev1.NodeGroup{}
		ng.Name = name
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: icName},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		return ng
	}

	// publishVersion rewrites instanceClassAPIVersion in the provider's registration Secret, the
	// way a cloud provider module would.
	publishVersion := func(version string) {
		GinkgoHelper()
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: cloudprovider.SecretNamespace, Name: cloudprovider.LegacySecretName,
		}, secret)).To(Succeed())
		if version == "" {
			delete(secret.Data, cloudprovider.InstanceClassAPIVersionKey)
		} else {
			secret.Data[cloudprovider.InstanceClassAPIVersionKey] = []byte(version)
		}
		Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
	}

	templatesOf := func(g Gomega, ngName string) []unstructured.Unstructured {
		list := &unstructured.UnstructuredList{}
		list.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha1")
		list.SetKind("DeckhouseMachineTemplateList")
		g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
			client.MatchingLabels{"node-group": ngName})).To(Succeed())
		return list.Items
	}

	createNodeGroup := func(name string) *deckhousev1.NodeGroup {
		GinkgoHelper()
		ng := newNodeGroup(name)
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ng) })
		return ng
	}

	BeforeEach(func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, newInstanceClass()))).To(Succeed())
		// The suite publishes v1alpha1; restore it so a spec that changed the version cannot
		// leak into the ones that follow.
		DeferCleanup(func() { publishVersion("v1alpha1") })
	})

	It("reads through the published version, whichever of the served ones it is", func() {
		By("the storage version renders a template")
		publishVersion("v1alpha1")
		alphaNG := createNodeGroup(testenv.UniqueName("icv-alpha"))
		var alphaTemplate string
		Eventually(func(g Gomega) string {
			items := templatesOf(g, alphaNG.Name)
			g.Expect(items).To(HaveLen(1))
			alphaTemplate = items[0].GetName()
			return alphaTemplate
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).ShouldNot(BeEmpty())

		By("the other served version renders one too")
		publishVersion("v1")
		servedNG := createNodeGroup(testenv.UniqueName("icv-served"))
		Eventually(func(g Gomega) int { return len(templatesOf(g, servedNG.Name)) },
			testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Equal(1))

		By("the first NodeGroup's template keeps its name — the same object read through either " +
			"served version hashes the same, so nothing may be renamed")
		Consistently(func(g Gomega) string {
			items := templatesOf(g, alphaNG.Name)
			g.Expect(items).To(HaveLen(1))
			return items[0].GetName()
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Equal(alphaTemplate))
	})

	// The regression. resolveInstanceClassVersion used to answer an unusable version with a
	// hardcoded v1alpha1 and let the render continue, so a wrong version was indistinguishable
	// from a right one — it just produced a different checksum, a different template name and a
	// rollout. Refusing to render is the only safe answer.
	It("renders nothing when the published version cannot serve the read", func() {
		publishVersion("v1beta1") // served by no InstanceClass CRD

		ng := createNodeGroup(testenv.UniqueName("icv-unserved"))

		Consistently(func(g Gomega) int { return len(templatesOf(g, ng.Name)) },
			testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(BeZero(),
			"an unserved version must stop the rendering, never fall back to one that works")

		By("and the NodeGroup renders as soon as a usable version is published, proving the " +
			"NodeGroup itself was fine and only the version was in the way")
		publishVersion("v1alpha1")
		Eventually(func(g Gomega) int { return len(templatesOf(g, ng.Name)) },
			testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Equal(1))
	})

	It("renders nothing until the cloud provider publishes a version at all", func() {
		publishVersion("")

		ng := createNodeGroup(testenv.UniqueName("icv-absent"))

		Consistently(func(g Gomega) int { return len(templatesOf(g, ng.Name)) },
			testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(BeZero(),
			"a provider that has not registered yet must not be guessed for")

		publishVersion("v1alpha1")
		Eventually(func(g Gomega) int { return len(templatesOf(g, ng.Name)) },
			testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Equal(1))
	})
})
