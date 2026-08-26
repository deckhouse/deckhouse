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

package bootstrapsecrets

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/bootstrap"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: as an operator adding a static node by hand, I take bootstrap.sh
// from the manual-bootstrap-for-<ng> secret — it has to appear without helm and
// be refreshed when the token rotates.
var _ = Describe("Bootstrap secrets controller", func() {
	It("writes manual-bootstrap-for-<ng> for a Static NodeGroup", func() {
		name := testenv.UniqueName("static")
		createNodeGroup(staticNodeGroup(name))

		secret := &corev1.Secret{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, manualSecretKey(name), secret)).To(Succeed())

			// Three keys — the contract dhctl (entity/node.go:125), CAPS
			// (client/bootstrap.go:485) and the documentation read.
			g.Expect(secret.Data).To(HaveKey("cloud-config"))
			g.Expect(secret.Data).To(HaveKey("bootstrap.sh"))
			g.Expect(secret.Data).To(HaveKey("apiserverEndpoints"))

			// The script's own tail, from bootstrap/script.go: proof a render ran.
			g.Expect(string(secret.Data["bootstrap.sh"])).To(ContainSubstring("get_phase2 | bash"))
			// This one is rendered by a ConfigMap template
			// (01-bootstrap-prerequisites.sh.tpl:26), so it proves the templates
			// travelled and that the cluster inputs reached them.
			g.Expect(string(secret.Data["bootstrap.sh"])).To(ContainSubstring(testClusterUUID))
			// A YAML list of the discovered endpoints, with no trailing newline:
			// helm's toYaml trimmed it, and dhctl compares these bytes.
			g.Expect(string(secret.Data["apiserverEndpoints"])).To(MatchRegexp(`\A- \d+\.\d+\.\d+\.\d+:\d+\z`))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("carrying the bootstrap token of the group")
		tokens, err := nodecommon.BootstrapTokens(suiteCtx, k8sClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(tokens).To(HaveKey(name))
		Expect(string(secret.Data["bootstrap.sh"])).To(ContainSubstring(tokens[name]))
	})

	// The packages-proxy token reaches the script only through the branch taken
	// when no apiserver endpoint was discovered (01-bootstrap-prerequisites.sh.tpl:
	// 28-33), which envtest never takes, so a reader that started returning nothing
	// would stay green. The other three are asserted here because they come from
	// the same buildInput pass and cost nothing to check.
	It("collects the packages-proxy token no rendered script can show", func() {
		ng := staticNodeGroup(testenv.UniqueName("inputs"))
		r := &Reconciler{
			context:       &bashiblecontext.Service{Client: k8sClient, Reader: k8sClient},
			derivedStatus: &derived_status.Service{Client: k8sClient},
		}
		r.Client = k8sClient

		resolved, validationErr, err := r.derivedStatus.ResolveNodeGroup(suiteCtx, ng)
		Expect(err).NotTo(HaveOccurred())
		Expect(validationErr).To(BeEmpty())

		in, err := r.buildInput(suiteCtx, ng, resolved)
		Expect(err).NotTo(HaveOccurred())

		Expect(in.PackagesProxy).To(HaveKeyWithValue("token", testPackagesProxyToken))
		Expect(in.Images).To(HaveKey("registrypackages"))
		Expect(in.MingetB64).To(Equal(base64.StdEncoding.EncodeToString([]byte("minget"))))
		Expect(in.ClusterUUID).To(Equal(testClusterUUID))
	})

	It("does not write a manual secret for a CloudEphemeral group", func() {
		name := testenv.UniqueName("cloud")
		ng := staticNodeGroup(name)
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "does-not-matter"},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		createNodeGroup(ng)

		Consistently(func() bool {
			secret := &corev1.Secret{}
			err := k8sClient.Get(suiteCtx, manualSecretKey(name), secret)
			return err == nil
		}, negativeCheckDuration, eventuallyPoll).Should(BeFalse())
	})

	// The other half of "no Secret, and here is why": a group the cloud checks
	// reject. The fixture takes the one path that needs no InstanceClass CRD —
	// a provider that published a kind but not the apiVersion to read it at
	// (derived_status/validate.go:40) — and is scoped to this spec so the rest
	// of the suite keeps running in a cluster with no cloud provider.
	It("says on the NodeGroup why a rejected group gets no secret", func() {
		create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: nodecommon.KubeSystemNamespace,
				Name:      ngcommon.CloudProviderSecretName,
			},
			Data: map[string][]byte{"type": []byte(`"dvp"`), nodecommon.InstanceClassKindKey: []byte("DVPInstanceClass")},
		})
		DeferCleanup(func() {
			Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: nodecommon.KubeSystemNamespace,
					Name:      ngcommon.CloudProviderSecretName,
				},
			}))).To(Succeed())
		})

		name := testenv.UniqueName("rejected")
		ng := staticNodeGroup(name)
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "missing"},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		createNodeGroup(ng)

		Eventually(func(g Gomega) {
			g.Expect(warningEventMessages(name, eventReasonSkipped)).
				To(ContainElement(ContainSubstring(nodecommon.InstanceClassAPIVersionKey)))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	// A candi update arrives as a chart upgrade, which only rewrites this
	// ConfigMap. Without the watch on it the Secrets would keep handing out the
	// previous bootstrap script until the next NodeGroup event.
	It("re-renders the secrets when the candi templates change", func() {
		name := testenv.UniqueName("templates")
		createNodeGroup(staticNodeGroup(name))

		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(name), secret)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		marker := "# candi-update-" + name
		cm := &corev1.ConfigMap{}
		cmKey := types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: bootstrap.TemplatesConfigMapName}
		Expect(k8sClient.Get(suiteCtx, cmKey, cm)).To(Succeed())
		original := cm.Data["lib.sh.tpl"]
		cm.Data["lib.sh.tpl"] = original + "\n" + marker + "\n"
		Expect(k8sClient.Update(suiteCtx, cm)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.ConfigMap{}
			Expect(k8sClient.Get(suiteCtx, cmKey, restored)).To(Succeed())
			restored.Data["lib.sh.tpl"] = original
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, manualSecretKey(name), secret)).To(Succeed())
			g.Expect(string(secret.Data["bootstrap.sh"])).To(ContainSubstring(marker))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})

	// Helm held this gate as `clusterUUID | required`: with an empty UUID rpp-get
	// queries the proxy at the prefix-less path, gets a 404, and the node hangs
	// for the whole StaticInstanceBootstrapTimeout instead of failing loudly.
	It("refuses to write anything while the cluster UUID is empty", func() {
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{Namespace: nodecommon.KubeSystemNamespace, Name: clusterUUIDConfigMapName}
		Expect(k8sClient.Get(suiteCtx, key, cm)).To(Succeed())
		Expect(k8sClient.Delete(suiteCtx, cm)).To(Succeed())
		DeferCleanup(func() {
			createClusterUUID()
		})

		name := testenv.UniqueName("no-uuid")
		createNodeGroup(staticNodeGroup(name))

		Consistently(func() bool {
			secret := &corev1.Secret{}
			err := k8sClient.Get(suiteCtx, manualSecretKey(name), secret)
			return err == nil
		}, negativeCheckDuration, eventuallyPoll).Should(BeFalse())

		By("saying why on the NodeGroup itself, not only in the controller log")
		Eventually(func(g Gomega) {
			g.Expect(warningEventMessages(name, eventReasonFailed)).To(ContainElement(ContainSubstring("cluster UUID is empty")))
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
	})
})

// warningEventMessages returns the messages of the Warning events recorded on the
// NodeGroup under the given reason. Listed across all namespaces on purpose: the
// recorder files a cluster-scoped object's events under "default", and pinning
// that here would make the helper fail silently if it ever changed.
func warningEventMessages(ngName, reason string) []string {
	GinkgoHelper()
	events := &corev1.EventList{}
	Expect(k8sClient.List(suiteCtx, events, client.InNamespace(""))).To(Succeed())

	var messages []string
	for i := range events.Items {
		e := &events.Items[i]
		if e.Type != corev1.EventTypeWarning || e.Reason != reason || e.InvolvedObject.Name != ngName {
			continue
		}
		messages = append(messages, e.Message)
	}
	return messages
}

func staticNodeGroup(name string) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       deckhousev1.NodeGroupSpec{NodeType: deckhousev1.NodeTypeStatic},
	}
}

func createNodeGroup(ng *deckhousev1.NodeGroup) {
	GinkgoHelper()
	Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, ng))).To(Succeed())
	})
}

func manualSecretKey(ngName string) types.NamespacedName {
	return types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: manualSecretPrefix + ngName}
}
