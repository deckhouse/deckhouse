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

package draining

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// createSpotNode creates the node in one shot: adding the spot label with a follow-up Update races
// the controller's own cordon/annotation patch and fails with a conflict.
func createSpotNode(name string, annotations map[string]string) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				nodecommon.NodeGroupLabel: "worker",
				spotTerminationLabel:      "true",
			},
			Annotations: annotations,
		},
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
}

func createInstance(name string) {
	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Create(suiteCtx, instance)).To(Succeed())
}

func instanceExists(name string) bool {
	instance := &deckhousev1alpha2.Instance{}
	err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, instance)
	if apierrors.IsNotFound(err) {
		return false
	}
	Expect(err).NotTo(HaveOccurred())
	// envtest runs no garbage collector, so an object under deletion never actually disappears;
	// a deletionTimestamp is the observable outcome of the controller's delete.
	return instance.DeletionTimestamp == nil
}

// User story: As a user running spot instances, I want the Instance of a spot node to be deleted
// once the node has been drained, so that the cloud VM is released only after workloads have been
// moved off it.
var _ = Describe("Draining controller, spot instance deletion", func() {
	It("deletes the Instance of a node that is spot-terminating and already drained", func() {
		name := testenv.UniqueName("spot-drained")
		createInstance(name)
		createSpotNode(name, map[string]string{nodecommon.DrainedAnnotation: "aws-spot"})

		Eventually(func() bool {
			return instanceExists(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
	})

	It("drains a spot-terminating node first and only then deletes its Instance", func() {
		name := testenv.UniqueName("spot-chain")
		createInstance(name)
		createSpotNode(name, map[string]string{nodecommon.DrainingAnnotation: "aws-spot"})

		// The whole chain: the controller cordons and drains the node, flips draining -> drained,
		// and the drained annotation is what releases the Instance.
		Eventually(func(g Gomega) {
			state := getNodeState(name)
			g.Expect(state.Spec.Unschedulable).To(BeTrue())
			g.Expect(state.Annotations).To(HaveKeyWithValue(nodecommon.DrainedAnnotation, "aws-spot"))
			g.Expect(state.Annotations).NotTo(HaveKey(nodecommon.DrainingAnnotation))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		Eventually(func() bool {
			return instanceExists(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())
	})

	It("keeps the Instance of a drained node that is not spot-terminating", func() {
		name := testenv.UniqueName("plain-drained")
		createInstance(name)
		createGroupNode(name, "worker", map[string]string{nodecommon.DrainedAnnotation: "bashible"})

		// Positive control: a spot-terminating node created alongside does lose its Instance.
		control := testenv.UniqueName("plain-control")
		createInstance(control)
		createSpotNode(control, map[string]string{nodecommon.DrainedAnnotation: "aws-spot"})
		Eventually(func() bool {
			return instanceExists(control)
		}, eventuallyTimeout, eventuallyPoll).Should(BeFalse())

		Expect(instanceExists(name)).To(BeTrue())
	})

	It("keeps the Instance of a spot-terminating node that has not been drained yet", func() {
		name := testenv.UniqueName("spot-undrained")
		createInstance(name)
		createSpotNode(name, nil)

		Consistently(func() bool {
			return instanceExists(name)
		}, negativeCheckDuration, eventuallyPoll).Should(BeTrue())
	})
})
