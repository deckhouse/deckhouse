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

package crdmigration

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	sigsyaml "sigs.k8s.io/yaml"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

var _ = Describe("MigrateNodeConfigs", func() {
	It("makes a NodeConfig with the string requestedBy of before the contract freeze readable", func(ctx SpecContext) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		data, err := os.ReadFile(testenv.NodeManagerCRDPaths(testenv.NodeConfigCRDFile)[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(sigsyaml.Unmarshal(data, crd)).To(Succeed())
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["extensions"].Items.Schema.
			Properties["requestedBy"] = apiextensionsv1.JSONSchemaProps{Type: "string"}
		Expect(k8sClient.Create(ctx, crd)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, crd))).To(Succeed())
		})
		Expect(envtest.WaitForCRDs(cfg, []*apiextensionsv1.CustomResourceDefinition{crd},
			envtest.CRDInstallOptions{MaxTime: time.Minute, PollInterval: 100 * time.Millisecond})).To(Succeed())

		digest := "sha256:0000000000000000000000000000000000000000000000000000000000000001"
		Expect(k8sClient.Create(ctx, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": internalv1alpha1.GroupVersion.String(),
			"kind":       "NodeConfig",
			"metadata":   map[string]any{"name": "legacy"},
			"spec": map[string]any{
				"nodeName":   "legacy",
				"systemType": "Immutable",
				"osImage":    map[string]any{"digest": digest},
				"extensions": []any{
					map[string]any{"name": "nodelet", "digest": digest, "requestedBy": "node-manager"},
					map[string]any{"name": "demo", "digest": digest, "requestedBy": "NodeExtensionRequest/demo"},
				},
			},
		}})).To(Succeed())

		Expect(k8sClient.List(ctx, &internalv1alpha1.NodeConfigList{})).
			To(MatchError(ContainSubstring("cannot unmarshal string")), "the read node-controller's cache fails on")

		Expect(MigrateNodeConfigs(ctx, k8sClient)).To(Succeed())

		list := &internalv1alpha1.NodeConfigList{}
		Expect(k8sClient.List(ctx, list)).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
		Expect(list.Items[0].Spec.Extensions).To(HaveLen(2))
		for _, extension := range list.Items[0].Spec.Extensions {
			Expect(extension.RequestedBy).To(BeNil(), extension.Name)
		}
	})
})
