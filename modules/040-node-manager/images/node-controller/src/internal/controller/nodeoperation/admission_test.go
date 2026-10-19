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

package nodeoperation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The policies under test in templates/node-controller/validation.yaml.
const (
	policyName           = "nodeoperations-own-node-only.deckhouse.io"
	disruptionPolicyName = "nodeoperations-disruption-by-node-manager.deckhouse.io"
)

// disruptionRefusal is the start of what the API server answers a Disruption
// created by anyone but node-manager.
const disruptionRefusal = "Disruption operations are created by node-manager"

// User story: As a cluster operator, I want a node to be able to report on its
// own operation and do nothing else with it, so no node can approve its own
// disruption or decide how an operation ended.
var _ = Describe("The policy that keeps a node to its own report", Ordered, func() {
	var (
		nodeName string
		asNode   client.Client
	)

	BeforeAll(func(ctx context.Context) {
		installPolicy(ctx, policyName)
		// Wider than the node-agent role in rbac-for-us.yaml on purpose: the
		// policy is the boundary even when RBAC lets a verb through.
		grantNodes(ctx, []rbacv1.PolicyRule{{
			APIGroups: []string{"deckhouse.io"},
			Resources: []string{"nodeoperations", "nodeoperations/status"},
			Verbs:     []string{"get", "list", "watch", "update", "patch"},
		}})

		nodeName = createNode(ctx, testenv.UniqueName("vap"), false).Name
		impersonated := rest.CopyConfig(cfg)
		impersonated.Impersonate = rest.ImpersonationConfig{
			UserName: "system:node:" + nodeName,
			Groups:   []string{"system:nodes", "system:authenticated"},
		}
		var err error
		asNode, err = client.New(impersonated, client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())

		// The API server loads a new policy asynchronously. A write the policy
		// would refuse probes for it; while it is not loaded yet the write lands
		// harmlessly, on a neighbour's operation the specs never look at.
		probe := createOperation(ctx, createNode(ctx, testenv.UniqueName("probe"), false).Name,
			v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			body := `{"metadata":{"labels":{"test.deckhouse.io/probe":"` + testenv.UniqueName("p") + `"}}}`
			g.Expect(asNode.Patch(ctx, probe, mergePatch(body))).
				To(MatchError(ContainSubstring("may only update its own NodeOperation")))
		}, time.Minute, testenv.EventuallyPoll).Should(Succeed())
	})

	It("refuses a node approving its own disruption", func(ctx context.Context) {
		op := createApproval(ctx, nodeName, false)

		Expect(asNode.Patch(ctx, op, mergePatch(`{"spec":{"approved":true}}`))).
			To(MatchError(ContainSubstring("approving it included")))
		Expect(getOperation(ctx, Default, op.Name).Spec.Approved).To(HaveValue(BeFalse()))
	})

	It("refuses a node writing the phase itself", func(ctx context.Context) {
		op := createOperation(ctx, nodeName, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(asNode.Status().Patch(ctx, op, mergePatch(`{"status":{"phase":"Completed"}}`))).
			To(MatchError(ContainSubstring("writes only the NodeReported condition")))
	})

	// The boot ID is what node-manager tells a finished reboot by.
	It("refuses a node writing the boot ID itself", func(ctx context.Context) {
		op := createOperation(ctx, nodeName, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		Expect(asNode.Status().Patch(ctx, op, mergePatch(`{"status":{"bootID":"forged"}}`))).
			To(MatchError(ContainSubstring("writes only the NodeReported condition")))
	})

	// An agent that does not remember its report reboots again on an InProgress
	// Reboot after boot. Refusing its second report is what stops it.
	It("lets a node report RebootStarted once", func(ctx context.Context) {
		op := createOperation(ctx, nodeName, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})
		Eventually(func(g Gomega) {
			g.Expect(getOperation(ctx, g, op.Name).Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		report := func() error {
			fresh := &v1alpha1.NodeOperation{}
			if err := asNode.Get(ctx, client.ObjectKeyFromObject(op), fresh); err != nil {
				return err
			}
			meta.SetStatusCondition(&fresh.Status.Conditions, nodeReport("RebootStarted", "The node is rebooting"))
			return asNode.Status().Update(ctx, fresh)
		}
		Eventually(report, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("node-manager recording the reboot on the reported operation")
		Eventually(func(g Gomega) {
			fresh := getOperation(ctx, g, op.Name)
			g.Expect(fresh.Status.Phase).To(Equal(v1alpha1.NodeOperationPhaseInProgress))
			g.Expect(fresh.Status.Reason).To(Equal("Rebooting"))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node, booted again, reporting the same reboot")
		Eventually(report, testenv.EventuallyTimeout, testenv.EventuallyPoll).
			Should(MatchError(ContainSubstring("may not write to an operation it reported RebootStarted on")))
	})

	It("refuses a node reporting on another node's operation", func(ctx context.Context) {
		other := createNode(ctx, testenv.UniqueName("neighbour"), false)
		op := createOperation(ctx, other.Name, v1alpha1.NodeOperationTypeReboot, &v1alpha1.NodeOperationDrainSpec{Skip: true})

		Eventually(func(g Gomega) {
			fresh := &v1alpha1.NodeOperation{}
			g.Expect(asNode.Get(ctx, client.ObjectKeyFromObject(op), fresh)).To(Succeed())
			meta.SetStatusCondition(&fresh.Status.Conditions, nodeReport("Succeeded", "not mine"))
			g.Expect(asNode.Status().Update(ctx, fresh)).To(MatchError(ContainSubstring("may only update its own NodeOperation")))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})
})

// User story: As a cluster operator, I want only node-manager to create a
// Disruption, so that no node is interrupted for a revision it never asked for.
var _ = Describe("The policy that leaves creating a Disruption to node-manager", Ordered, func() {
	var asOperator, asNodeController client.Client

	BeforeAll(func(ctx context.Context) {
		// Registered before the policy, so it runs after the policy is deleted:
		// the other specs create Disruptions as the admin, and the API server
		// unloads a deleted policy asynchronously.
		DeferCleanup(func(ctx context.Context) {
			Eventually(func(g Gomega) {
				op := disruptionOn(testenv.UniqueName("after"))
				g.Expect(k8sClient.Create(ctx, op)).To(Succeed())
				g.Expect(k8sClient.Delete(ctx, op)).To(Succeed())
			}, time.Minute, testenv.EventuallyPoll).Should(Succeed())
		})
		installPolicy(ctx, disruptionPolicyName)

		// Both are cluster administrators, so RBAC lets the create through and
		// only the policy decides.
		asOperator = clientAs("operator", "system:masters", "system:authenticated")
		asNodeController = clientAs("system:serviceaccount:d8-cloud-instance-manager:node-controller",
			"system:masters", "system:serviceaccounts", "system:authenticated")

		// The API server loads a new policy asynchronously.
		Eventually(func(g Gomega) {
			op := disruptionOn(testenv.UniqueName("probe"))
			DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })
			g.Expect(asOperator.Create(ctx, op)).To(MatchError(ContainSubstring(disruptionRefusal)))
		}, time.Minute, testenv.EventuallyPoll).Should(Succeed())
	})

	It("refuses an operator creating a Disruption", func(ctx context.Context) {
		op := disruptionOn(testenv.UniqueName("by-hand"))
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		Expect(asOperator.Create(ctx, op)).To(MatchError(ContainSubstring(disruptionRefusal)))
	})

	It("lets node-controller create a Disruption", func(ctx context.Context) {
		op := disruptionOn(testenv.UniqueName("by-node-manager"))
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		Expect(asNodeController.Create(ctx, op)).To(Succeed())
	})

	It("lets an operator create a Reboot", func(ctx context.Context) {
		op := &v1alpha1.NodeOperation{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("reboot")},
			Spec:       v1alpha1.NodeOperationSpec{Type: v1alpha1.NodeOperationTypeReboot, NodeName: testenv.UniqueName("worker")},
		}
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, op) })

		Expect(asOperator.Create(ctx, op)).To(Succeed())
	})
})

// disruptionOn is a Disruption the way node-manager writes it, for a node the
// specs never create.
func disruptionOn(nodeName string) *v1alpha1.NodeOperation {
	return &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("disruption")},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationTypeDisruption,
			NodeName:         nodeName,
			ConfigGeneration: ptr.To(int64(1)),
			NodeConfigUID:    uuid.NewUUID(),
			Approved:         ptr.To(false),
		},
	}
}

func clientAs(userName string, groups ...string) client.Client {
	GinkgoHelper()

	impersonated := rest.CopyConfig(cfg)
	impersonated.Impersonate = rest.ImpersonationConfig{UserName: userName, Groups: groups}
	c, err := client.New(impersonated, client.Options{Scheme: k8sClient.Scheme()})
	Expect(err).NotTo(HaveOccurred())
	return c
}

// installPolicy installs the policy and its binding as the module template
// renders them; the Helm expressions in those two documents are only the name
// and the module labels.
func installPolicy(ctx context.Context, name string) {
	GinkgoHelper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "..", "templates", "node-controller", "validation.yaml"))
	Expect(err).NotTo(HaveOccurred())
	start := strings.Index(string(raw), `$policyName := "`+name+`"`)
	Expect(start).To(BeNumerically(">=", 0), "the policy is in the template")
	section := string(raw[start:])
	// The binding ends where the next policy of the file begins.
	if end := strings.Index(section, "\n---\n{{- $policyName"); end >= 0 {
		section = section[:end]
	}

	var rendered []string
	for _, line := range strings.Split(section, "\n")[1:] {
		line = strings.ReplaceAll(line, "{{ $policyName }}", name)
		if strings.Contains(line, "{{") {
			continue
		}
		rendered = append(rendered, line)
	}
	docs := strings.Split(strings.Join(rendered, "\n"), "\n---\n")
	Expect(docs).To(HaveLen(2))

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	Expect(yaml.UnmarshalStrict([]byte(docs[0]), policy)).To(Succeed())
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
	Expect(yaml.UnmarshalStrict([]byte(docs[1]), binding)).To(Succeed())

	for _, obj := range []client.Object{policy, binding} {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, obj) })
	}
}

// grantNodes gives every kubelet the rules through a ClusterRole bound to the
// system:nodes group.
func grantNodes(ctx context.Context, rules []rbacv1.PolicyRule) {
	GinkgoHelper()

	name := testenv.UniqueName("nodes")
	role := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: rules}
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: name},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: "system:nodes"}},
	}
	for _, obj := range []client.Object{role, binding} {
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, obj) })
	}
}
