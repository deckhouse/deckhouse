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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

		token, err := EnsureToken(suiteCtx, k8sClient, ng.Name)
		Expect(err).NotTo(HaveOccurred())

		in, err := BuildInput(suiteCtx, r.context, resolved, token)
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

	// An immutable group gets no bootstrap Secret here, yet nodebootstrap renders the
	// group's token into every machine's userdata (nodebootstrap/render.go:45) and mints
	// none: mintToken is the only creator of a bootstrap-token Secret left in the repo.
	It("mints a bootstrap token for an immutable CloudEphemeral group", func() {
		name := testenv.UniqueName("immutable")
		ng := staticNodeGroup(name)
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.SystemType = deckhousev1.SystemTypeImmutable
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "does-not-matter"},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		createNodeGroup(ng)

		Eventually(func(g Gomega) {
			tokens := &corev1.SecretList{}
			g.Expect(k8sClient.List(suiteCtx, tokens,
				client.InNamespace(nodecommon.KubeSystemNamespace),
				client.MatchingLabels{nodecommon.BootstrapTokenNodeGroupLabel: name})).To(Succeed())
			g.Expect(tokens.Items).NotTo(BeEmpty())
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
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

	// The digests land literally inside every bootstrap.sh and every release rewrites
	// them. Same argument as the candi templates above: nothing else enqueues a
	// NodeGroup on this ConfigMap, so a stale digest would stand until the resync.
	It("re-renders the secrets when the image digests change", func() {
		name := testenv.UniqueName("digests")
		createNodeGroup(staticNodeGroup(name))

		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(name), secret)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		digest := "sha256:jq-" + name
		cm := &corev1.ConfigMap{}
		cmKey := types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: imagesDigestsConfigMapName}
		Expect(k8sClient.Get(suiteCtx, cmKey, cm)).To(Succeed())
		original := cm.Data[imagesDigestsKey]
		cm.Data[imagesDigestsKey] = strings.Replace(original, `"sha256:jq"`, `"`+digest+`"`, 1)
		Expect(cm.Data[imagesDigestsKey]).NotTo(Equal(original),
			"the fixture must still carry the digest this spec rewrites")
		Expect(k8sClient.Update(suiteCtx, cm)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.ConfigMap{}
			Expect(k8sClient.Get(suiteCtx, cmKey, restored)).To(Succeed())
			restored.Data[imagesDigestsKey] = original
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})

		// The digest reaches the script through the rpp-get install line of
		// 01-bootstrap-prerequisites.sh.tpl:37, so a stale render cannot show it.
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, manualSecretKey(name), secret)).To(Succeed())
			g.Expect(string(secret.Data["bootstrap.sh"])).To(ContainSubstring(digest))
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

// User story: as an operator who deleted a NodeGroup, I do not want its bootstrap
// Secret left behind. Helm used to prune it; the keep annotation this migration
// stamps ahead of the handover took that duty away and left it here.
var _ = Describe("Bootstrap secret cleanup", func() {
	It("deletes the bootstrap secret of a NodeGroup that is gone", func() {
		gone := testenv.UniqueName("gone")
		live := testenv.UniqueName("live")
		createNodeGroup(staticNodeGroup(gone))
		createNodeGroup(staticNodeGroup(live))

		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(gone), secret)
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(live), &corev1.Secret{})
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		// The label is how the sweep finds the Secret once the NodeGroup that names
		// it is gone: the CAPI Secret's name carries no group name to parse.
		Expect(secret.Labels).To(HaveKeyWithValue(ngcommon.MachineDeploymentNodeGroupLabel, gone))

		Expect(k8sClient.Delete(suiteCtx, &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: gone},
		})).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(suiteCtx, manualSecretKey(gone), &corev1.Secret{}))
		}, eventuallyTimeout, eventuallyPoll).Should(BeTrue())

		By("leaving the secret of the NodeGroup that is still there")
		Expect(k8sClient.Get(suiteCtx, manualSecretKey(live), &corev1.Secret{})).To(Succeed())
	})

	// The two ways this sweep could take down a running cluster, on one pass it
	// controls: deleting a Secret an operator made by hand, and deleting the
	// Secret of a NodeGroup that is still there.
	It("deletes only the secrets of NodeGroups that are gone", func() {
		live := testenv.UniqueName("kept")
		createNodeGroup(staticNodeGroup(live))
		absent := testenv.UniqueName("absent")

		orphan := labelledSecret("orphan-"+absent, map[string]string{
			"heritage": "deckhouse", "module": "node-manager",
			ngcommon.MachineDeploymentNodeGroupLabel: absent,
		})
		byHand := labelledSecret("by-hand-"+absent, map[string]string{
			ngcommon.MachineDeploymentNodeGroupLabel: absent,
		})
		ofLiveGroup := labelledSecret("machine-class-"+live, map[string]string{
			"heritage": "deckhouse", "module": "node-manager",
			ngcommon.MachineDeploymentNodeGroupLabel: live,
		})
		// deckhouse-registry and bashible-bashbooster wear the module labels and
		// belong to no NodeGroup: nothing in this namespace may go for lack of a
		// node-group label alone.
		ofNoGroup := labelledSecret("of-no-group-"+absent, map[string]string{
			"heritage": "deckhouse", "module": "node-manager",
		})

		Expect(CollectOrphanedSecrets(suiteCtx, k8sClient, k8sClient)).To(Succeed())

		Expect(apierrors.IsNotFound(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(orphan), &corev1.Secret{}))).
			To(BeTrue(), "the secret of a NodeGroup that is gone must be collected")
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(byHand), &corev1.Secret{})).
			To(Succeed(), "a secret without the module labels was not written here and is not ours to delete")
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(ofLiveGroup), &corev1.Secret{})).
			To(Succeed(), "the secret of a NodeGroup that still exists must survive every sweep")
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(ofNoGroup), &corev1.Secret{})).
			To(Succeed(), "a module secret that belongs to no NodeGroup is not this sweep's business")
	})

	// registry-packages-proxy-token is a legacy ServiceAccount token: helm creates the
	// shell, kube-controller-manager fills it moments later, and a node that bootstraps in
	// between bakes in PACKAGES_PROXY_TOKEN=passthrough for good. Nothing else enqueues a
	// NodeGroup on that Secret, so without the watch the empty reading would stand until
	// the 30-minute resync — the window is cluster install.
	//
	// The token itself is not observable: it reaches the script only through the branch
	// taken when no apiserver endpoint was found, which envtest never takes. So the spec
	// deletes the rendered Secret and asks whether the group is reconciled again at all.
	It("re-renders when the packages-proxy token is filled in", func() {
		name := testenv.UniqueName("rpp-token")
		createNodeGroup(staticNodeGroup(name))
		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(name), &corev1.Secret{})
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed())

		By("removing the rendered secret and confirming nothing else brings it back")
		Expect(k8sClient.Delete(suiteCtx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: nodecommon.MachineNamespace, Name: manualSecretPrefix + name},
		})).To(Succeed())
		Consistently(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(suiteCtx, manualSecretKey(name), &corev1.Secret{}))
		}, negativeCheckDuration, eventuallyPoll).Should(BeTrue())

		By("filling the token, the way kube-controller-manager does")
		token := &corev1.Secret{}
		tokenKey := types.NamespacedName{
			Namespace: nodecommon.MachineNamespace,
			Name:      bashiblecontext.PackagesProxyTokenSecretName,
		}
		Expect(k8sClient.Get(suiteCtx, tokenKey, token)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.Secret{}
			Expect(k8sClient.Get(suiteCtx, tokenKey, restored)).To(Succeed())
			restored.Data = map[string][]byte{"token": []byte(testPackagesProxyToken)}
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})
		token.Data = map[string][]byte{"token": []byte("filled-in-by-kube-controller-manager")}
		Expect(k8sClient.Update(suiteCtx, token)).To(Succeed())

		Eventually(func() error {
			return k8sClient.Get(suiteCtx, manualSecretKey(name), &corev1.Secret{})
		}, eventuallyTimeout, eventuallyPoll).Should(Succeed(),
			"a change to the packages-proxy token must re-render the bootstrap secrets")
	})
})

// labelledSecret creates a Secret in the machine namespace with exactly the given
// labels, so a spec can state what the sweep is allowed to select on.
func labelledSecret(name string, labels map[string]string) *corev1.Secret {
	GinkgoHelper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nodecommon.MachineNamespace,
			Name:      name,
			Labels:    labels,
		},
		Data: map[string][]byte{"cloud-config": []byte("#cloud-config")},
	}
	Expect(k8sClient.Create(suiteCtx, secret)).To(Succeed())
	DeferCleanup(func() {
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, secret))).To(Succeed())
	})
	return secret
}
