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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var _ = Describe("instance-class checksum ConfigMap watch", func() {
	var (
		ngName string
		ng     *deckhousev1.NodeGroup
		cm     *corev1.ConfigMap
	)

	templateNameFor := func(checksum string) string {
		return ngName + "-" + sha256Hash(envtestClusterUUID+envtestZone+checksum)
	}

	mdTemplateName := func() string {
		md := &unstructured.Unstructured{}
		md.SetGroupVersionKind(schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeployment"})
		mdName := ngName + "-" + sha256Hash(envtestClusterUUID+envtestZone)
		if err := k8sClient.Get(suiteCtx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, md); err != nil {
			return ""
		}
		name, _, _ := unstructured.NestedString(md.Object, "spec", "template", "spec", "infrastructureRef", "name")
		return name
	}

	BeforeEach(func() {
		ngName = testenv.UniqueName("cm-watch")

		// The ConfigMap exists before the NodeGroup so the very first reconcile already has a checksum.
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: common.InstanceClassChecksumConfigMapName, Namespace: common.MachineNamespace},
			Data:       map[string]string{ngName: "one"},
		}
		Expect(k8sClient.Create(suiteCtx, cm)).To(Succeed())

		ng = &deckhousev1.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{Name: ngName},
			Spec: deckhousev1.NodeGroupSpec{
				NodeType: deckhousev1.NodeTypeCloudEphemeral,
				CloudInstances: &deckhousev1.CloudInstancesSpec{
					Zones:          []string{envtestZone},
					MinPerZone:     1,
					MaxPerZone:     1,
					ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: "ubuntu"},
				},
			},
		}
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		// A merge patch: the reconciler adds its finalizer concurrently, an Update would conflict.
		base := ng.DeepCopy()
		ng.Status.Engine = engineCAPI
		Expect(k8sClient.Status().Patch(suiteCtx, ng, client.MergeFrom(base))).To(Succeed())

		Eventually(mdTemplateName, 20*time.Second, 200*time.Millisecond).Should(Equal(templateNameFor("one")))
	})

	AfterEach(func() {
		testenv.RemoveFinalizers(suiteCtx, k8sClient, ng)
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, ng))).To(Succeed())
		Expect(client.IgnoreNotFound(k8sClient.Delete(suiteCtx, cm))).To(Succeed())
	})

	It("moves the MachineDeployment to the new template when only the ConfigMap changes", func() {
		Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		cm.Data[ngName] = "two"
		Expect(k8sClient.Update(suiteCtx, cm)).To(Succeed())

		// No NodeGroup event happens here: only the ConfigMap watch can wake the reconciler.
		Eventually(mdTemplateName, 20*time.Second, 200*time.Millisecond).Should(Equal(templateNameFor("two")))
	})
})
