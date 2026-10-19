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

package instance_controller

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// createNodeConfig creates the NodeConfig of a node with the given last report of its
// node agent, as the agent publishes it in status.lastReconcileTime.
func createNodeConfig(name string, systemType internalv1alpha1.SystemType, lastReport time.Time) {
	config := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       internalv1alpha1.NodeSpec{NodeName: name, SystemType: systemType},
	}
	if systemType == internalv1alpha1.SystemTypeImmutable {
		config.Spec.OSImage = internalv1alpha1.OSImage{Digest: "sha256:" + strings.Repeat("a", 64)}
	}
	Expect(k8sClient.Create(suiteCtx, config)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, config))).To(Succeed())
	})

	config.Status.LastReconcileTime = metav1.NewTime(lastReport)
	Expect(k8sClient.Status().Update(suiteCtx, config)).To(Succeed())
}

var _ = Describe("Node agent of an Immutable node (NodeConfig status.lastReconcileTime)", func() {
	It("marks a silent node agent with reason NoReport and a message without bashible", func() {
		name := uniqueName("agent-silent")
		lastReport := time.Now().Add(-11 * time.Minute).Truncate(time.Second)
		createNodeConfig(name, internalv1alpha1.SystemTypeImmutable, lastReport)
		newNodeBackedInstance(name)

		want := "the node agent has not reported since " + lastReport.UTC().Format(time.RFC3339)
		Eventually(func(g Gomega) {
			instance := getInstance(name)
			agent := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeNodeAgentReady)
			g.Expect(agent).NotTo(BeNil())
			g.Expect(agent.Status).To(Equal(metav1.ConditionUnknown))
			g.Expect(agent.Reason).To(Equal("NoReport"))
			g.Expect(agent.Message).To(Equal(want))
			g.Expect(instance.Status.Message).To(Equal(want))
			g.Expect(strings.ToLower(instance.Status.Message)).NotTo(ContainSubstring("bashible"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("marks a reporting node agent True", func() {
		name := uniqueName("agent-reporting")
		createNodeConfig(name, internalv1alpha1.SystemTypeImmutable, time.Now())
		newNodeBackedInstance(name)

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			agent := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeNodeAgentReady)
			g.Expect(agent).NotTo(BeNil())
			g.Expect(agent.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(agent.Reason).To(Equal("Reporting"))
			g.Expect(instance.Status.Message).To(Equal("the node agent reports"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("marks a silent node agent on a bashible node too, above a healthy bashible", func() {
		name := uniqueName("agent-bashible")
		createNodeConfig(name, internalv1alpha1.SystemTypeMutable, time.Now().Add(-11*time.Minute))
		newNodeBackedInstance(name)
		applyBashibleCondition(name, bashibleReadyManager,
			bashibleReady(metav1.ConditionTrue, "StepsCompleted", "converge cycle finished", time.Now()))

		Eventually(func(g Gomega) {
			instance := getInstance(name)
			agent := conditionOf(instance, deckhousev1alpha2.InstanceConditionTypeNodeAgentReady)
			g.Expect(agent).NotTo(BeNil())
			g.Expect(agent.Reason).To(Equal("NoReport"))
			g.Expect(instance.Status.Message).To(HavePrefix("the node agent has not reported since"))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
