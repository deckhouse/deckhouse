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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"

	// Registers the bootstrap-secrets controller in this test binary. The spec
	// below is here rather than in that package's own suite because the cloud
	// provider registration a CAPI engine needs already lives in this fixture.
	_ "github.com/deckhouse/node-controller/internal/controller/bootstrapsecrets"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// The whole risk of moving the bootstrap render out of helm is a Secret written
// under the wrong name: a CAPI Machine reads exactly the name its
// MachineDeployment carries, and a mismatch means nodes that never join.
var _ = Describe("CAPI bootstrap secret", func() {
	It("writes the cloud-init under the name the MachineDeployment carries", func() {
		By("publishing the candi templates and the image digests the render reads")
		testenv.EnsureObject(suiteCtx, k8sClient, testenv.BootstrapTemplatesConfigMap())
		testenv.EnsureObject(suiteCtx, k8sClient, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: common.MachineNamespace, Name: "bashible-apiserver-files"},
			Data: map[string]string{"images_digests.json": `{"registrypackages":{"jq171":"sha256:jq",` +
				`"d8Curl891":"sha256:curl","tailLog":"sha256:tail","rppGet":"sha256:rpp"}}`},
		})

		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName("bootstrap-secret-ic")
		Expect(unstructured.SetNestedMap(ic.Object, map[string]interface{}{
			"cpu":    map[string]interface{}{"cores": int64(2), "coreFraction": "100%"},
			"memory": map[string]interface{}{"size": "4Gi"},
		}, "spec", "virtualMachine")).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(suiteCtx, ic))).To(Succeed())

		ng := &deckhousev1.NodeGroup{}
		ng.Name = testenv.UniqueName("capi-bootstrap")
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: ic.GetName()},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{"zone-a"},
		}
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ng) })

		var dataSecretName string
		Eventually(func() error {
			mdList := &capiv1beta2.MachineDeploymentList{}
			if err := k8sClient.List(suiteCtx, mdList, client.InNamespace(common.MachineNamespace),
				client.MatchingLabels{"node-group": ng.Name}); err != nil {
				return err
			}
			if len(mdList.Items) != 1 {
				return fmt.Errorf("expected 1 MachineDeployment, got %d", len(mdList.Items))
			}
			name := mdList.Items[0].Spec.Template.Spec.Bootstrap.DataSecretName
			if name == nil || *name == "" {
				return fmt.Errorf("MachineDeployment carries no dataSecretName")
			}
			dataSecretName = *name
			return nil
		}, 20*time.Second, 250*time.Millisecond).Should(Succeed())

		By("the Secret exists under exactly that name, in the CAPI bootstrap shape")
		secret := &corev1.Secret{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: common.MachineNamespace, Name: dataSecretName,
			}, secret)).To(Succeed())

			// The two keys a CAPI bootstrap data Secret is read by.
			g.Expect(string(secret.Data["format"])).To(Equal("cloud-config"))
			g.Expect(string(secret.Data["value"])).To(HavePrefix("#cloud-config\n"))
			// Rendered from the candi templates, not a placeholder.
			g.Expect(string(secret.Data["value"])).To(ContainSubstring("get_phase2 | bash"))
		}, 20*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})
