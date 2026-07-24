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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bootstrapv1alpha1 "github.com/deckhouse/node-controller/api/bootstrap.deckhouse.io/v1alpha1"
	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

const (
	cloudInstanceManagerNS = "d8-cloud-instance-manager"
	testKubernetesVersion  = "1.35"
	testContainerdDigest   = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	testCNIDigest          = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testKubeletDigest      = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	testClusterCA          = "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n"
)

// User story: As a cluster operator, I want every immutable machine to boot from
// bootstrap data that already carries its own node name, so that no placeholder
// has to be substituted on the node and any infrastructure provider can hand the
// data over unchanged.
var _ = Describe("NodeBootstrap controller", func() {
	BeforeEach(func(ctx context.Context) {
		ensureClusterInputs(ctx)
	})

	It("renders per-machine bootstrap data with the node name filled in", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		createImmutableNodeGroup(ctx, ngName)
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

	// Bootstrap is consumed once: rotating the group token must not churn a live
	// machine's userdata, or the machine could be handed data it never booted with.
	It("does not re-render an existing secret", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		createImmutableNodeGroup(ctx, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := createBootstrapConfig(ctx, machine)

		secretName := machine.Name + dataSecretSuffix
		var original string
		Eventually(func(g Gomega) {
			original = string(getSecret(ctx, g, secretName).Data[secretValueKey])
			g.Expect(original).NotTo(BeEmpty())
		}, testenv.EventuallyTimeout, testenv.EventuallyPoll).Should(Succeed())

		By("nudging the controller to reconcile the config again")
		fresh := getConfig(ctx, Default, config.Name)
		fresh.Annotations = map[string]string{"test.deckhouse.io/nudge": "1"}
		Expect(k8sClient.Update(ctx, fresh)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(string(getSecret(ctx, g, secretName).Data[secretValueKey])).To(Equal(original))
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("does nothing until the config has an owner Machine", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		createImmutableNodeGroup(ctx, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)

		// A clone the MachineSet has not re-parented onto the Machine yet.
		orphan := &bootstrapv1alpha1.NodeBootstrapConfig{
			ObjectMeta: metav1.ObjectMeta{Name: testenv.UniqueName("cfg"), Namespace: cloudInstanceManagerNS},
		}
		Expect(k8sClient.Create(ctx, orphan)).To(Succeed())
		DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, orphan) })

		secretName := machine.Name + dataSecretSuffix
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: secretName}, &corev1.Secret{})
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
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: secretName}, &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})

	It("skips a paused config", func(ctx context.Context) {
		ngName := testenv.UniqueName("imm")
		createImmutableNodeGroup(ctx, ngName)
		ensureBootstrapToken(ctx, ngName)

		machine := createMachine(ctx, testenv.UniqueName("m"), ngName)
		config := &bootstrapv1alpha1.NodeBootstrapConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:        testenv.UniqueName("cfg"),
				Namespace:   cloudInstanceManagerNS,
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
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: secretName}, &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, testenv.NegativeCheckDuration, testenv.EventuallyPoll).Should(Succeed())
	})
})

func getSecret(ctx context.Context, g Gomega, name string) *corev1.Secret {
	secret := &corev1.Secret{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: name}, secret)).To(Succeed())
	return secret
}

func getConfig(ctx context.Context, g Gomega, name string) *bootstrapv1alpha1.NodeBootstrapConfig {
	config := &bootstrapv1alpha1.NodeBootstrapConfig{}
	g.Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: name}, config)).To(Succeed())
	return config
}

func createImmutableNodeGroup(ctx context.Context, name string) *deckhousev1.NodeGroup {
	GinkgoHelper()

	ng := &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType:   deckhousev1.NodeTypeCloudEphemeral,
			SystemType: deckhousev1.SystemTypeImmutable,
			CloudInstances: &deckhousev1.CloudInstancesSpec{
				MinPerZone:     1,
				MaxPerZone:     3,
				ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
			},
		},
	}
	Expect(k8sClient.Create(ctx, ng)).To(Succeed())
	DeferCleanup(func(ctx context.Context) { _ = k8sClient.Delete(ctx, ng) })

	fresh := &deckhousev1.NodeGroup{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, fresh)).To(Succeed())
	fresh.Status.KubernetesVersion = testKubernetesVersion
	Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
	return fresh
}

func createMachine(ctx context.Context, name, ngName string) *capiv1beta2.Machine {
	GinkgoHelper()

	machine := &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cloudInstanceManagerNS,
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
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: cloudInstanceManagerNS, Name: name}, fresh)).To(Succeed())
	return fresh
}

func createBootstrapConfig(ctx context.Context, machine *capiv1beta2.Machine) *bootstrapv1alpha1.NodeBootstrapConfig {
	GinkgoHelper()

	config := &bootstrapv1alpha1.NodeBootstrapConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testenv.UniqueName("cfg"),
			Namespace: cloudInstanceManagerNS,
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

	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      testenv.UniqueName("bootstrap-token"),
			Labels:    map[string]string{bootstrapTokenNGLabel: ngName},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"token-id":     []byte("abcdef"),
			"token-secret": []byte("0123456789abcdef"),
			"expiration":   []byte(time.Now().Add(24 * time.Hour).Format(time.RFC3339)),
		},
	})
}

// ensureClusterInputs creates the cluster state the bootstrap userdata is
// rendered from, the same inputs the day-2 NodeConfig is built from.
func ensureClusterInputs(ctx context.Context) {
	GinkgoHelper()

	ensureNamespace(ctx, kubeSystemNS)
	ensureNamespace(ctx, cloudInstanceManagerNS)

	digests := fmt.Sprintf(`{"registrypackages":{"containerdSysext224":%q,"kubernetesCniSysext162":%q,"kubeletSysext1356":%q}}`,
		testContainerdDigest, testCNIDigest, testKubeletDigest)
	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: "bashible-apiserver-files"},
		Data:       map[string]string{"images_digests.json": digests},
	})

	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: cloudInstanceManagerNS, Name: "registry-packages-proxy-token"},
		Data:       map[string][]byte{"token": []byte("proxy-token")},
	})

	ensureObject(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: "kube-root-ca.crt"},
		Data:       map[string]string{"ca.crt": testClusterCA},
	})

	ensureObject(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: kubeSystemNS, Name: "d8-cluster-configuration"},
		Data: map[string][]byte{
			"cluster-configuration.yaml": []byte("apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterDomain: cluster.local\nkubernetesVersion: \"" + testKubernetesVersion + ".6\"\n"),
		},
	})

	// envtest publishes its own apiserver in the default/kubernetes EndpointSlice;
	// rendering refuses without at least one API server endpoint.
	slice := &discoveryv1.EndpointSlice{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, slice)).To(Succeed())
	Expect(slice.Endpoints).NotTo(BeEmpty(), "envtest should publish its apiserver endpoint")

	dnsService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kubeSystemNS,
			Name:      "kube-dns",
			Labels:    map[string]string{"k8s-app": "kube-dns"},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 53, Protocol: corev1.ProtocolUDP}}},
	}
	ensureObject(ctx, dnsService)
}

func ensureNamespace(ctx context.Context, name string) {
	GinkgoHelper()
	ensureObject(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})
}

func ensureObject(ctx context.Context, obj client.Object) {
	GinkgoHelper()
	err := k8sClient.Create(ctx, obj)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}
