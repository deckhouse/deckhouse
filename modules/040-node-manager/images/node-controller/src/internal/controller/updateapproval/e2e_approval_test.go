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

package updateapproval

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

var _ = Describe("Update approval", func() {
	It("approves a node waiting for approval", func() {
		ng := uniqueName("approve")
		createNodeGroup(ng, nil)
		setChecksum(ng, "ng-checksum")
		node := uniqueName("approve-node")
		createReadyNode(node, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.WaitingForApprovalAnnotation:    "",
		})

		Eventually(func(g Gomega) {
			got := nodeState(node)
			g.Expect(hasAnnotation(got, ua.ApprovedAnnotation)).To(BeTrue())
			g.Expect(hasAnnotation(got, ua.WaitingForApprovalAnnotation)).To(BeFalse())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("approves only maxConcurrent nodes at once", func() {
		ng := uniqueName("concurrency")
		createNodeGroup(ng, func(g *v1.NodeGroup) {
			one := intOrString(1)
			g.Spec.Update = &v1.UpdateSpec{MaxConcurrent: &one}
		})
		setChecksum(ng, "ng-checksum")
		n1 := uniqueName("conc-node")
		n2 := uniqueName("conc-node")
		createReadyNode(n1, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.WaitingForApprovalAnnotation:    "",
		})
		createReadyNode(n2, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.WaitingForApprovalAnnotation:    "",
		})

		// Exactly one node is approved while maxConcurrent=1: assert it converges to 1 and
		// stays there (the second is not approved until the first finishes).
		Eventually(func() int {
			return approvedCount(n1, n2)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(1))
		Consistently(func() int {
			return approvedCount(n1, n2)
		}, negativeCheckDuration, eventuallyPoll).Should(Equal(1))
	})

	It("automatically approves a disruption when no drain is required", func() {
		ng := uniqueName("disruption-auto")
		createNodeGroup(ng, func(g *v1.NodeGroup) {
			no := false
			g.Spec.Disruptions = &v1.DisruptionsSpec{
				ApprovalMode: v1.DisruptionApprovalModeAutomatic,
				Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: &no},
			}
		})
		setChecksum(ng, "ng-checksum")
		node := uniqueName("disruption-auto-node")
		createReadyNode(node, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.ApprovedAnnotation:              "",
			ua.DisruptionRequiredAnnotation:    "",
		})

		Eventually(func(g Gomega) {
			got := nodeState(node)
			g.Expect(hasAnnotation(got, ua.DisruptionApprovedAnnotation)).To(BeTrue())
			g.Expect(hasAnnotation(got, ua.DisruptionRequiredAnnotation)).To(BeFalse())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("starts draining a node when drain is required before disruption approval", func() {
		ng := uniqueName("disruption-drain")
		createNodeGroup(ng, func(g *v1.NodeGroup) {
			yes := true
			g.Spec.Disruptions = &v1.DisruptionsSpec{
				ApprovalMode: v1.DisruptionApprovalModeAutomatic,
				Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: &yes},
			}
		})
		// Ready >= 2 so NeedDrainNode does not short-circuit the drain.
		setReadyStatus(ng, 3, 3)
		setChecksum(ng, "ng-checksum")
		node := uniqueName("disruption-drain-node")
		createReadyNode(node, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.ApprovedAnnotation:              "",
			ua.DisruptionRequiredAnnotation:    "",
		})

		Eventually(func(g Gomega) {
			got := nodeState(node)
			g.Expect(got.Annotations).To(HaveKeyWithValue(ua.DrainingAnnotation, "bashible"))
			g.Expect(hasAnnotation(got, ua.DisruptionApprovedAnnotation)).To(BeFalse())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	It("does not approve a disruption in manual mode", func() {
		ng := uniqueName("disruption-manual")
		createNodeGroup(ng, func(g *v1.NodeGroup) {
			g.Spec.Disruptions = &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeManual}
		})
		setChecksum(ng, "ng-checksum")
		node := uniqueName("disruption-manual-node")
		createReadyNode(node, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "old-checksum",
			ua.ApprovedAnnotation:              "",
			ua.DisruptionRequiredAnnotation:    "",
		})

		// Positive control: an up-to-date sibling node (approved + matching checksum + ready) in
		// the same group gets its approval annotations cleaned up, proving the controller has
		// reconciled this group — so the manual node's unchanged disruption-required state is
		// meaningful, not just not-yet-reconciled. It uses the cleanup path, which does not touch
		// the manual node or consume an update slot.
		control := uniqueName("disruption-manual-control")
		createReadyNode(control, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "ng-checksum",
			ua.ApprovedAnnotation:              "",
		})
		Eventually(func() bool {
			return !hasAnnotation(nodeState(control), ua.ApprovedAnnotation)
		}, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		Consistently(func(g Gomega) {
			got := nodeState(node)
			g.Expect(hasAnnotation(got, ua.DisruptionRequiredAnnotation)).To(BeTrue())
			g.Expect(hasAnnotation(got, ua.DisruptionApprovedAnnotation)).To(BeFalse())
		}, negativeCheckDuration, eventuallyPoll).Should(Succeed())
	})

	It("cleans up approval annotations when a node becomes up to date", func() {
		ng := uniqueName("cleanup")
		createNodeGroup(ng, nil)
		setChecksum(ng, "ng-checksum")
		node := uniqueName("cleanup-node")
		createReadyNode(node, ng, map[string]string{
			ua.ConfigurationChecksumAnnotation: "ng-checksum",
			ua.ApprovedAnnotation:              "",
			ua.DisruptionApprovedAnnotation:    "",
		})

		Eventually(func(g Gomega) {
			got := nodeState(node)
			g.Expect(hasAnnotation(got, ua.ApprovedAnnotation)).To(BeFalse())
			g.Expect(hasAnnotation(got, ua.DisruptionApprovedAnnotation)).To(BeFalse())
			g.Expect(hasAnnotation(got, ua.WaitingForApprovalAnnotation)).To(BeFalse())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})
