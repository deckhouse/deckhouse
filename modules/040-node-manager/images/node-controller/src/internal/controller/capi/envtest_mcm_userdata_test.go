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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/bootstrap"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The MCM machine-class Secret is built from the same three inputs as the bootstrap Secrets, so
// this controller must watch what bootstrapsecrets watches. Nothing else re-enqueues a NodeGroup
// on them, and the resync (10 minutes) is far beyond the timeout these specs wait for.
var _ = Describe("MCM machine-class userData freshness", func() {
	const (
		mcmZone = "zone-a"
		// negativeCheck is how long "nothing brings it back" is observed; the resync that
		// eventually would is 10 minutes away.
		negativeCheck = 3 * time.Second
	)

	// A YandexMachineClass because that kind ships in the MCM CRD file every suite installs;
	// the provider stays dvp, so the InstanceClass and the render fixtures of the suite hold.
	const mcmMachineClassFixture = `apiVersion: machine.sapcloud.io/v1alpha1
kind: YandexMachineClass
metadata:
  name: {{ .nodeGroup.name }}-{{ printf "%v%v" .Values.global.discovery.clusterUUID .zoneName | sha256sum | trunc 8 }}
  namespace: d8-cloud-instance-manager
spec:
  zoneID: {{ .zoneName }}
  secretRef:
    name: {{ .nodeGroup.name }}-{{ printf "%v%v" .Values.global.discovery.clusterUUID .zoneName | sha256sum | trunc 8 }}
    namespace: d8-cloud-instance-manager
`

	const mcmProviderConfigFixture = `folderID: {{ "myfolder" | b64enc }}`

	registrationKey := types.NamespacedName{Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName}
	templatesKey := types.NamespacedName{Namespace: common.MachineNamespace, Name: bootstrap.TemplatesConfigMapName}
	digestsKey := types.NamespacedName{Namespace: common.MachineNamespace, Name: bootstrap.ImagesDigestsConfigMapName}
	tokenKey := types.NamespacedName{
		Namespace: common.MachineNamespace,
		Name:      bashiblecontext.PackagesProxyTokenSecretName,
	}

	machineClassSecretKey := func(ngName string) types.NamespacedName {
		return types.NamespacedName{
			Namespace: common.MachineNamespace,
			Name:      ngName + "-" + sha256Hash(suiteClusterUUID+mcmZone),
		}
	}

	userData := func(key types.NamespacedName) func(Gomega) string {
		return func(g Gomega) string {
			secret := &corev1.Secret{}
			g.Expect(k8sClient.Get(suiteCtx, key, secret)).To(Succeed())
			return string(secret.Data["userData"])
		}
	}

	BeforeEach(func() {
		By("publishing the three inputs of the cloud-init render")
		testenv.EnsureObject(suiteCtx, k8sClient, testenv.BootstrapTemplatesConfigMap())
		testenv.EnsureObject(suiteCtx, k8sClient, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: digestsKey.Namespace, Name: digestsKey.Name},
			Data: map[string]string{"images_digests.json": `{"registrypackages":{"jq171":"sha256:jq",` +
				`"d8Curl891":"sha256:curl","tailLog":"sha256:tail","rppGet":"sha256:rpp"}}`},
		})
		// The app label is not decoration: the machine-namespace Secret informer selects on it
		// (common/cache.go), so without it neither the read nor the watch sees this Secret.
		testenv.EnsureObject(suiteCtx, k8sClient, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: tokenKey.Namespace,
				Name:      tokenKey.Name,
				Labels:    map[string]string{"app": "registry-packages-proxy"},
			},
			Data: map[string][]byte{"token": []byte("initial-token")},
		})

		By("publishing the provider's MCM templates")
		testenv.EnsureObject(suiteCtx, k8sClient, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: providerTemplateSecretNamespace, Name: "d8-cloud-provider-dvp-mcm"},
			Data: map[string][]byte{
				"machine-class.yaml":                         []byte(mcmMachineClassFixture),
				"machine-class.checksum":                     []byte(instanceClassChecksumFixture),
				"config-for-machine-controller-manager.yaml": []byte(mcmProviderConfigFixture),
			},
		})

		By("letting the registration serve MCM as well, and putting it back afterwards")
		registration := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, registrationKey, registration)).To(Succeed())
		registration.Data["machineClassKind"] = []byte("YandexMachineClass")
		Expect(k8sClient.Update(suiteCtx, registration)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.Secret{}
			Expect(k8sClient.Get(suiteCtx, registrationKey, restored)).To(Succeed())
			delete(restored.Data, "machineClassKind")
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})
	})

	// createMCMNodeGroup creates a CloudEphemeral group pinned to MCM by the use-mcm annotation
	// and waits until its machine-class Secret carries a rendered cloud-init.
	createMCMNodeGroup := func(name string) types.NamespacedName {
		GinkgoHelper()

		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName(name + "-ic")
		Expect(unstructured.SetNestedMap(ic.Object, map[string]interface{}{
			"cpu":    map[string]interface{}{"cores": int64(2), "coreFraction": "100%"},
			"memory": map[string]interface{}{"size": "4Gi"},
		}, "spec", "virtualMachine")).To(Succeed())
		Expect(k8sClient.Create(suiteCtx, ic)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ic) })

		ng := &deckhousev1.NodeGroup{}
		ng.Name = name
		ng.Annotations = map[string]string{"node.deckhouse.io/use-mcm": "true"}
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: ic.GetName()},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{mcmZone},
		}
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ng) })

		key := machineClassSecretKey(name)
		Eventually(userData(key), testenv.EventuallyTimeout, testenv.EventuallyPoll).
			ShouldNot(BeEmpty(), "the machine-class secret must carry a rendered cloud-init")
		return key
	}

	// A candi update arrives as a chart upgrade, which only rewrites this ConfigMap.
	It("re-renders when the candi templates change", func() {
		name := testenv.UniqueName("mcm-templates")
		key := createMCMNodeGroup(name)

		marker := "# candi-update-" + name
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(suiteCtx, templatesKey, cm)).To(Succeed())
		original := cm.Data["lib.sh.tpl"]
		cm.Data["lib.sh.tpl"] = original + "\n" + marker + "\n"
		Expect(k8sClient.Update(suiteCtx, cm)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.ConfigMap{}
			Expect(k8sClient.Get(suiteCtx, templatesKey, restored)).To(Succeed())
			restored.Data["lib.sh.tpl"] = original
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})

		Eventually(userData(key), testenv.EventuallyTimeout, testenv.EventuallyPoll).
			Should(ContainSubstring(marker), "a candi update must reach the machine-class userData")
	})

	// The digests are baked literally into every bootstrap.sh and every release rewrites them.
	It("re-renders when the image digests change", func() {
		name := testenv.UniqueName("mcm-digests")
		key := createMCMNodeGroup(name)

		digest := "sha256:jq-" + name
		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(suiteCtx, digestsKey, cm)).To(Succeed())
		original := cm.Data["images_digests.json"]
		cm.Data["images_digests.json"] = strings.Replace(original, `"sha256:jq"`, `"`+digest+`"`, 1)
		Expect(cm.Data["images_digests.json"]).NotTo(Equal(original),
			"the fixture must still carry the digest this spec rewrites")
		Expect(k8sClient.Update(suiteCtx, cm)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.ConfigMap{}
			Expect(k8sClient.Get(suiteCtx, digestsKey, restored)).To(Succeed())
			restored.Data["images_digests.json"] = original
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})

		// The digest reaches the script through the rpp-get install line of
		// 01-bootstrap-prerequisites.sh.tpl:37, so a stale render cannot show it.
		Eventually(userData(key), testenv.EventuallyTimeout, testenv.EventuallyPoll).
			Should(ContainSubstring(digest), "a release's digests must reach the machine-class userData")
	})

	// kube-controller-manager fills this token moments after helm creates the empty Secret. The
	// value is unobservable in envtest (01-bootstrap-prerequisites.sh.tpl:29 takes another branch),
	// so the spec removes the rendered Secret and asks whether the group is reconciled again at all.
	It("re-renders when the packages-proxy token is filled in", func() {
		name := testenv.UniqueName("mcm-rpp-token")
		key := createMCMNodeGroup(name)

		By("removing the machine-class secret and confirming nothing else brings it back")
		Expect(k8sClient.Delete(suiteCtx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		})).To(Succeed())
		Consistently(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(suiteCtx, key, &corev1.Secret{}))
		}, negativeCheck, testenv.EventuallyPoll).Should(BeTrue())

		By("filling the token, the way kube-controller-manager does")
		token := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, tokenKey, token)).To(Succeed())
		token.Data = map[string][]byte{"token": []byte("filled-in-by-kube-controller-manager")}
		Expect(k8sClient.Update(suiteCtx, token)).To(Succeed())
		DeferCleanup(func() {
			restored := &corev1.Secret{}
			Expect(k8sClient.Get(suiteCtx, tokenKey, restored)).To(Succeed())
			restored.Data = map[string][]byte{"token": []byte("initial-token")}
			Expect(k8sClient.Update(suiteCtx, restored)).To(Succeed())
		})

		Eventually(userData(key), testenv.EventuallyTimeout, testenv.EventuallyPoll).
			ShouldNot(BeEmpty(), "a change to the packages-proxy token must re-render the machine-class userData")
	})
})
