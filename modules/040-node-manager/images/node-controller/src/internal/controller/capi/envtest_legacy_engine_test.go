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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As an operator upgrading a cluster that predates the CAPI migration, I want a
// NodeGroup whose engine was never pinned to keep running on MCM, so that an upgrade does not
// silently recreate every machine on the other engine.
var _ = Describe("Live MCM MachineDeployments of a NodeGroup", func() {
	createMCMDeploymentIn := func(namespace, name, ngName string) {
		md := ngcommon.NewUnstructured(ngcommon.MCMMachineDeploymentGVK)
		md.SetName(name)
		md.SetNamespace(namespace)
		md.SetLabels(map[string]string{ngcommon.MachineDeploymentNodeGroupLabel: ngName})
		Expect(k8sClient.Create(suiteCtx, md)).To(Succeed())
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, md))).To(Succeed())
		})
	}

	createMCMDeployment := func(name, ngName string) {
		createMCMDeploymentIn(common.MachineNamespace, name, ngName)
	}

	It("reports true for a group that has one", func() {
		ngName := testenv.UniqueName("legacy-mcm")
		createMCMDeployment(ngName+"-nova", ngName)

		live, err := ngcommon.FindMachineDeployments(suiteCtx, k8sClient, ngName)
		Expect(err).NotTo(HaveOccurred())
		Expect(live.MCM).To(BeTrue())
	})

	It("reports false for a group that has none", func() {
		live, err := ngcommon.FindMachineDeployments(suiteCtx, k8sClient, testenv.UniqueName("no-mcm"))
		Expect(err).NotTo(HaveOccurred())
		Expect(live.MCM).To(BeFalse())
		Expect(live.CAPI).To(BeFalse())
	})

	It("does not count a deployment of the same group in another namespace", func() {
		ngName := testenv.UniqueName("foreign-ns")
		createMCMDeploymentIn("default", ngName+"-nova", ngName)

		live, err := ngcommon.FindMachineDeployments(suiteCtx, k8sClient, ngName)
		Expect(err).NotTo(HaveOccurred())
		Expect(live.MCM).To(BeFalse())
	})

	It("does not count another group's deployment", func() {
		other := testenv.UniqueName("other")
		createMCMDeployment(other+"-nova", other)

		live, err := ngcommon.FindMachineDeployments(suiteCtx, k8sClient, testenv.UniqueName("mine"))
		Expect(err).NotTo(HaveOccurred())
		Expect(live.MCM).To(BeFalse())
	})

	It("resolves an ambiguous engine to MCM when the group already runs on it", func() {
		ngName := testenv.UniqueName("ambiguous")
		createMCMDeployment(ngName+"-nova", ngName)

		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeCloudEphemeral},
		}
		registration := derived_status.CloudProviderRegistration{
			MachineClassKind: "YandexMachineClass",
			CAPIClusterKind:  "YandexCluster",
		}

		engine, err := derived_status.ResolveEngine(suiteCtx, k8sClient, ng, registration)
		Expect(err).NotTo(HaveOccurred())
		Expect(engine).To(Equal(engineMCM))
	})

	It("leaves an ambiguous engine at CAPI when no MCM deployment exists", func() {
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("fresh")},
			Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeCloudEphemeral},
		}
		registration := derived_status.CloudProviderRegistration{
			MachineClassKind: "YandexMachineClass",
			CAPIClusterKind:  "YandexCluster",
		}

		engine, err := derived_status.ResolveEngine(suiteCtx, k8sClient, ng, registration)
		Expect(err).NotTo(HaveOccurred())
		Expect(engine).To(Equal(engineCAPI))
	})

	It("keeps an explicit CAPI pin even while an MCM deployment is still around", func() {
		ngName := testenv.UniqueName("pinned")
		createMCMDeployment(ngName+"-nova", ngName)

		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeCloudEphemeral},
			Status:     deckhousev1.NodeGroupStatus{Engine: engineCAPI},
		}
		registration := derived_status.CloudProviderRegistration{
			MachineClassKind: "YandexMachineClass",
			CAPIClusterKind:  "YandexCluster",
		}

		engine, err := derived_status.ResolveEngine(suiteCtx, k8sClient, ng, registration)
		Expect(err).NotTo(HaveOccurred())
		Expect(engine).To(Equal(engineCAPI), "the pin outranks the machines")
	})
})
