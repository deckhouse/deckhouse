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

package nodebootstrap

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1alpha1 "github.com/deckhouse/node-controller/api/bootstrap.deckhouse.io/v1alpha1"
	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a cluster operator, I want every immutable machine to boot
// from bootstrap data that already carries its own node name, so any
// infrastructure provider can hand the data over unchanged.
var _ = Describe("NodeBootstrap controller", func() {
	BeforeEach(func(ctx context.Context) {
		testenv.EnsureClusterInputs(ctx, k8sClient)
	})

	It("renders per-machine bootstrap data with the node name filled in", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		testenv.CreateImmutableNodeGroup(ctx, k8sClient, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := createBootstrapConfig(ctx, machine)

		secretName := machine.Name + dataSecretSuffix
		Eventually(func(g Gomega) {
			secret := getSecret(ctx, g, secretName)

			// The infrastructure provider (capdvp) reads Data["value"] and is
			// told it is a cloud-config through Data["format"].
			g.Expect(string(secret.Data[secretFormatKey])).To(Equal(secretFormatCloudConfig))
			value := string(secret.Data[secretValueKey])
			g.Expect(value).To(HavePrefix("#cloud-config"))

			// The node name is baked in; the placeholder never reaches the wire.
			g.Expect(value).To(ContainSubstring("nodeName: " + machine.Name))
			g.Expect(value).To(ContainSubstring("hostname: " + machine.Name))
			g.Expect(value).NotTo(ContainSubstring("__NODE_NAME__"))

			// The token kubelet presents on first contact is the group's.
			g.Expect(value).To(ContainSubstring("bootstrapToken:"))

			// Only the desired state travels: omitempty does not drop a struct,
			// so marshalling the API type verbatim would put a
			// "status: {lastReconcileTime: null}" on every machine that boots.
			g.Expect(value).NotTo(ContainSubstring("status:"))
			g.Expect(value).NotTo(ContainSubstring("lastReconcileTime"))

			// The Secret is owned by the config, so it is collected with it.
			g.Expect(secret.OwnerReferences).To(HaveLen(1))
			g.Expect(secret.OwnerReferences[0].Kind).To(Equal(nodeBootstrapConfigKind))
			g.Expect(secret.OwnerReferences[0].Name).To(Equal(config.Name))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		// The status is the v1beta2 bootstrap contract the Machine controller waits
		// on before handing the userdata to the infrastructure provider.
		Eventually(func(g Gomega) {
			fresh := getConfig(ctx, g, config.Name)
			g.Expect(fresh.Status.DataSecretName).To(HaveValue(Equal(secretName)))
			g.Expect(fresh.Status.Initialization).NotTo(BeNil())
			g.Expect(fresh.Status.Initialization.DataSecretCreated).To(BeTrue())
			g.Expect(fresh.Status.Conditions).To(ContainElement(And(
				HaveField("Type", conditionDataSecretAvailable),
				HaveField("Status", metav1.ConditionTrue),
			)))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	// The provider builds the VM from the userdata, so once it reports the
	// infrastructure provisioned nothing will read the Secret again and
	// re-rendering would only burn uncached reads for the life of the Machine.
	It("stops re-rendering once the infrastructure is provisioned", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		testenv.CreateImmutableNodeGroup(ctx, k8sClient, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := createBootstrapConfig(ctx, machine)

		secretName := machine.Name + dataSecretSuffix
		Eventually(func(g Gomega) {
			g.Expect(getSecret(ctx, g, secretName).Data[secretValueKey]).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		setInfrastructureProvisioned(ctx, machine.Name)
		rotateBootstrapToken(ctx, ngName)
		nudgeConfig(ctx, config.Name)

		var frozen string
		Eventually(func(g Gomega) {
			frozen = string(getSecret(ctx, g, secretName).Data[secretValueKey])
			g.Expect(frozen).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		rotateBootstrapToken(ctx, ngName)
		nudgeConfig(ctx, config.Name)

		Consistently(func(g Gomega) {
			g.Expect(string(getSecret(ctx, g, secretName).Data[secretValueKey])).To(Equal(frozen))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	// Bootstrap is consumed once, but only once the node is there: the token
	// baked into the userdata expires in four hours, so a machine whose VM is
	// created late must not keep the copy it was first given.
	It("re-renders while the machine has no node, and freezes once it has one", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		testenv.CreateImmutableNodeGroup(ctx, k8sClient, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := createBootstrapConfig(ctx, machine)

		secretName := machine.Name + dataSecretSuffix
		var original string
		Eventually(func(g Gomega) {
			original = string(getSecret(ctx, g, secretName).Data[secretValueKey])
			g.Expect(original).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the group's token rotating before the machine ever reaches the API")
		rotateBootstrapToken(ctx, ngName)
		nudgeConfig(ctx, config.Name)

		Eventually(func(g Gomega) {
			g.Expect(string(getSecret(ctx, g, secretName).Data[secretValueKey])).NotTo(Equal(original))
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("the node registering, which is what makes the userdata history")
		setNodeRef(ctx, machine.Name)
		rotateBootstrapToken(ctx, ngName)
		nudgeConfig(ctx, config.Name)

		var frozen string
		Eventually(func(g Gomega) {
			frozen = string(getSecret(ctx, g, secretName).Data[secretValueKey])
			g.Expect(frozen).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		rotateBootstrapToken(ctx, ngName)
		nudgeConfig(ctx, config.Name)

		Consistently(func(g Gomega) {
			g.Expect(string(getSecret(ctx, g, secretName).Data[secretValueKey])).To(Equal(frozen))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("does nothing until the config has an owner Machine", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		testenv.CreateImmutableNodeGroup(ctx, k8sClient, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)

		// A clone the MachineSet has not re-parented onto the Machine yet.
		orphan := &bootstrapv1alpha1.NodeBootstrapConfig{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("cfg"), Namespace: nodecommon.MachineNamespace},
		}
		Expect(k8sClient.Create(ctx, orphan)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, orphan) })

		secretName := machine.Name + dataSecretSuffix
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: secretName}, &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())

		By("the MachineSet setting the Machine as the config owner")
		setOwnerMachine(ctx, orphan, machine)

		Eventually(func(g Gomega) {
			getSecret(ctx, g, secretName)
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
	})

	It("leaves a bashible-managed group alone", func(ctx context.Context) {
		ngName := testenv.UniqueName("mut")
		ng := &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType:   deckhousev1.NodeTypeCloudEphemeral,
				SystemType: deckhousev1.SystemTypeMutable,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					MinPerZone:     1,
					MaxPerZone:     3,
					ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, ng)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		createBootstrapConfig(ctx, machine)

		secretName := machine.Name + dataSecretSuffix
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: secretName}, &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("skips a paused config", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		testenv.CreateImmutableNodeGroup(ctx, k8sClient, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := &bootstrapv1alpha1.NodeBootstrapConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        testenv.UniqueName("cfg"),
				Namespace:   nodecommon.MachineNamespace,
				Annotations: map[string]string{capiv1beta2.PausedAnnotation: "true"},
				Labels:      map[string]string{machineNodeGroupLabel: ngName},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: capiv1beta2.GroupVersion.String(),
					Kind:       machineKind,
					Name:       machine.Name,
					UID:        machine.UID,
					Controller: ptr.To(true),
				}},
			},
		}
		Expect(k8sClient.Create(ctx, config)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, config) })

		secretName := machine.Name + dataSecretSuffix
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: secretName}, &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

func getSecret(ctx context.Context, g Gomega, name string) *corev1.Secret {
	secret := &corev1.Secret{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, secret)).To(Succeed())
	return secret
}

func getConfig(ctx context.Context, g Gomega, name string) *bootstrapv1alpha1.NodeBootstrapConfig {
	config := &bootstrapv1alpha1.NodeBootstrapConfig{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, config)).To(Succeed())
	return config
}

func createMachine(ctx context.Context, name, ngName string) *capiv1beta2.Machine {
	GinkgoHelper()

	machine := &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: nodecommon.MachineNamespace,
			Labels:    map[string]string{machineNodeGroupLabel: ngName},
		},
		Spec: capiv1beta2.MachineSpec{
			ClusterName: "test-cluster",
			// The Machine CRD requires both refs; the bootstrap controller keys
			// off the owner reference, not these, so placeholders are enough.
			Bootstrap: capiv1beta2.Bootstrap{DataSecretName: ptr.To("placeholder")},
			InfrastructureRef: capiv1beta2.ContractVersionedObjectReference{
				Kind:     "DeckhouseMachineTemplate",
				Name:     name + "-infra",
				APIGroup: "infrastructure.cluster.x-k8s.io",
			},
		},
	}
	Expect(k8sClient.Create(ctx, machine)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, machine) })

	fresh := &capiv1beta2.Machine{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, fresh)).To(Succeed())
	return fresh
}

func createBootstrapConfig(ctx context.Context, machine *capiv1beta2.Machine) *bootstrapv1alpha1.NodeBootstrapConfig {
	GinkgoHelper()

	config := &bootstrapv1alpha1.NodeBootstrapConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testenv.UniqueName("cfg"),
			Namespace: nodecommon.MachineNamespace,
			Labels:    map[string]string{machineNodeGroupLabel: machine.Labels[machineNodeGroupLabel]},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: capiv1beta2.GroupVersion.String(),
				Kind:       machineKind,
				Name:       machine.Name,
				UID:        machine.UID,
				Controller: ptr.To(true),
			}},
		},
	}
	Expect(k8sClient.Create(ctx, config)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, config) })
	return config
}

func setOwnerMachine(ctx context.Context, config *bootstrapv1alpha1.NodeBootstrapConfig, machine *capiv1beta2.Machine) {
	GinkgoHelper()

	fresh := getConfig(ctx, Default, config.Name)
	fresh.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: capiv1beta2.GroupVersion.String(),
		Kind:       machineKind,
		Name:       machine.Name,
		UID:        machine.UID,
		Controller: ptr.To(true),
	}}
	Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
}

// ensureBootstrapToken creates a per-group rotating bootstrap token, the same
// kind of secret order_bootstrap_token maintains for bashible nodes.
func ensureBootstrapToken(ctx context.Context, ngName string) {
	GinkgoHelper()

	testenv.EnsureObject(ctx, k8sClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nodecommon.KubeSystemNamespace,
			Name:      testenv.UniqueName("bootstrap-token"),
			Labels:    map[string]string{nodecommon.BootstrapTokenNodeGroupLabel: ngName},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"token-id":     []byte("abcdef"),
			"token-secret": []byte("0123456789abcdef"),
			"expiration":   []byte(time.Now().Add(24 * time.Hour).Format(time.RFC3339)),
		},
	})
}

// rotateBootstrapToken replaces the secret part of the group's token, the way
// order_bootstrap_token does once the current one is close to expiring.
func rotateBootstrapToken(ctx context.Context, ngName string) {
	GinkgoHelper()

	secrets := &corev1.SecretList{}
	Expect(k8sClient.List(ctx, secrets,
		client.InNamespace(nodecommon.KubeSystemNamespace),
		client.MatchingLabels{nodecommon.BootstrapTokenNodeGroupLabel: ngName},
	)).To(Succeed())
	Expect(secrets.Items).To(HaveLen(1))

	secret := &secrets.Items[0]
	patch := client.MergeFrom(secret.DeepCopy())
	secret.Data["token-secret"] = []byte(testenv.UniqueName("tok"))
	Expect(k8sClient.Patch(ctx, secret, patch)).To(Succeed())
}

// setNodeRef plays the part of the CAPI Machine controller noticing that the
// machine's kubelet has registered a Node.
func setNodeRef(ctx context.Context, machineName string) {
	GinkgoHelper()

	machine := &capiv1beta2.Machine{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: machineName}, machine)).To(Succeed())
	machine.Status.NodeRef = capiv1beta2.MachineNodeReference{Name: machineName}
	Expect(k8sClient.Status().Update(ctx, machine)).To(Succeed())
}

// setInfrastructureProvisioned plays the part of the infrastructure provider
// reporting that it has built the machine's VM — from the userdata, which it
// therefore will not read again.
func setInfrastructureProvisioned(ctx context.Context, machineName string) {
	GinkgoHelper()

	machine := &capiv1beta2.Machine{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: machineName}, machine)).To(Succeed())
	machine.Status.Initialization.InfrastructureProvisioned = ptr.To(true)
	Expect(k8sClient.Status().Update(ctx, machine)).To(Succeed())
}

func nudgeConfig(ctx context.Context, name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		config := getConfig(ctx, g, name)
		patch := client.MergeFrom(config.DeepCopy())
		if config.Annotations == nil {
			config.Annotations = map[string]string{}
		}
		config.Annotations["test.deckhouse.io/nudge"] = testenv.UniqueName("n")
		g.Expect(k8sClient.Patch(ctx, config, patch)).To(Succeed())
	}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())
}
