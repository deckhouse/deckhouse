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

package nodeuser

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	eventuallyTimeout     = testenv.EventuallyTimeout
	eventuallyPoll        = testenv.EventuallyPoll
	negativeCheckDuration = testenv.NegativeCheckDuration
)

// createNode creates a node; withGroupLabel controls the node.deckhouse.io/group label, which is
// what marks a node as Deckhouse-managed and therefore a legitimate key in status.errors.
func createNode(name string, withGroupLabel bool) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if withGroupLabel {
		node.Labels = map[string]string{common.NodeGroupLabel: "worker"}
	}
	Expect(k8sClient.Create(suiteCtx, node)).To(Succeed())
}

func deleteNode(name string) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
	Expect(k8sClient.Delete(suiteCtx, node)).To(Succeed())
}

// nodeUserSpec is the spec every NodeUser in this suite is created with. The controller touches
// status.errors only, so specs assert the object comes back through the typed client unharmed.
// sshPublicKey is left empty on purpose: the CRD's oneOf rejects a spec carrying both key fields.
func nodeUserSpec() deckhousev1.NodeUserSpec {
	return deckhousev1.NodeUserSpec{
		UID: 1100,
		SSHPublicKeys: []string{
			"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIexampleexampleexampleexampleexample",
			"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIsecondsecondsecondsecondsecondx",
		},
		PasswordHash: "fake-password-hash",
		IsSudoer:     true,
		NodeGroups:   []string{"master", "worker"},
		ExtraGroups:  []string{"docker"},
	}
}

// createNodeUser creates a NodeUser and then writes its status.errors, which lives on the status
// subresource and is therefore dropped by the Create call.
func createNodeUser(name string, statusErrors map[string]string) {
	nodeUser := &deckhousev1.NodeUser{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       nodeUserSpec(),
	}
	Expect(k8sClient.Create(suiteCtx, nodeUser)).To(Succeed())

	if len(statusErrors) == 0 {
		return
	}

	nodeUser.Status.Errors = statusErrors
	Expect(k8sClient.Status().Update(suiteCtx, nodeUser)).To(Succeed())
}

func getNodeUser(name string) *deckhousev1.NodeUser {
	nodeUser := &deckhousev1.NodeUser{}
	Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, nodeUser)).To(Succeed())
	return nodeUser
}

func statusErrorsOf(name string) map[string]string {
	return getNodeUser(name).Status.Errors
}

func resourceVersionOf(name string) string {
	return getNodeUser(name).ResourceVersion
}

var _ = AfterEach(func() {
	list := &deckhousev1.NodeUserList{}
	Expect(k8sClient.List(suiteCtx, list)).To(Succeed())
	for i := range list.Items {
		Expect(k8sClient.Delete(suiteCtx, &list.Items[i])).To(Succeed())
	}

	nodes := &corev1.NodeList{}
	Expect(k8sClient.List(suiteCtx, nodes)).To(Succeed())
	for i := range nodes.Items {
		Expect(k8sClient.Delete(suiteCtx, &nodes.Items[i])).To(Succeed())
	}
})

// User story: As an administrator managing users through NodeUser, I want errors that refer to
// nodes which no longer exist to disappear from the resource status, so that the error count I see
// reflects only nodes that are actually failing.
var _ = Describe("NodeUserErrors controller", func() {
	It("clears an error for a node that does not exist and keeps the rest", func() {
		live := testenv.UniqueName("live")
		createNode(live, true)

		name := testenv.UniqueName("mixed")
		createNodeUser(name, map[string]string{live: "cannot create user", "vanished-node": "cannot create user"})

		Eventually(func() map[string]string {
			return statusErrorsOf(name)
		}, eventuallyTimeout, eventuallyPoll).Should(Equal(map[string]string{live: "cannot create user"}))
	})

	It("leaves status.errors an empty map after the last error goes, and the spec untouched", func() {
		name := testenv.UniqueName("lastkey")
		createNodeUser(name, map[string]string{"vanished-node": "cannot create user"})

		Eventually(func() map[string]string {
			return statusErrorsOf(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeEmpty())

		// status.errors stays addressable after the last key goes; the CRD default `{}`
		// (crds/nodeuser.yaml) is what restores it, so this does not pin the patch shape —
		// TestStaleErrorsPatch does.
		nodeUser := getNodeUser(name)
		Expect(nodeUser.Status.Errors).NotTo(BeNil())
		Expect(nodeUser.Spec).To(Equal(nodeUserSpec()))
	})

	It("clears an error for a node without the node-group label, which is not a Deckhouse node", func() {
		unmanaged := testenv.UniqueName("unmanaged")
		createNode(unmanaged, false)

		name := testenv.UniqueName("unlabelled")
		createNodeUser(name, map[string]string{unmanaged: "cannot create user"})

		Eventually(func() map[string]string {
			return statusErrorsOf(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeEmpty())
	})

	It("clears the error after the node it refers to is deleted", func() {
		doomed := testenv.UniqueName("doomed")
		createNode(doomed, true)

		name := testenv.UniqueName("ondelete")
		createNodeUser(name, map[string]string{doomed: "cannot create user"})
		Consistently(func() map[string]string {
			return statusErrorsOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(HaveKey(doomed))

		deleteNode(doomed)

		Eventually(func() map[string]string {
			return statusErrorsOf(name)
		}, eventuallyTimeout, eventuallyPoll).Should(BeEmpty())
	})

	It("does not write the NodeUser when every error refers to a live node", func() {
		live := testenv.UniqueName("allgood")
		createNode(live, true)

		name := testenv.UniqueName("nowrite")
		createNodeUser(name, map[string]string{live: "cannot create user"})
		settled := resourceVersionOf(name)

		Consistently(func() string {
			return resourceVersionOf(name)
		}, negativeCheckDuration, eventuallyPoll).Should(Equal(settled))
		Expect(statusErrorsOf(name)).To(HaveKey(live))
	})

	It("does not write a NodeUser that has no errors at all", func() {
		name := testenv.UniqueName("noerrors")
		createNodeUser(name, nil)
		settled := resourceVersionOf(name)

		// Positive control: a NodeUser with a stale error created alongside does get cleaned, so
		// the untouched resourceVersion above is a decision and not an idle controller.
		control := testenv.UniqueName("noerrors-control")
		createNodeUser(control, map[string]string{"vanished-node": "cannot create user"})
		Eventually(func() map[string]string {
			return statusErrorsOf(control)
		}, eventuallyTimeout, eventuallyPoll).Should(BeEmpty())

		Expect(resourceVersionOf(name)).To(Equal(settled))
	})
})
