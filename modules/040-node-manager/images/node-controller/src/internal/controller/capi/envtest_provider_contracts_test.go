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
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a user of any cloud provider Deckhouse ships, I want the upgrade that moves my
// provider to the v2 contract to leave my machines exactly where they are.
//
// The specs in envtest_machinetemplate_v2_test.go prove the machinery on a synthetic contract.
// These prove the same thing on the file each provider really ships: the contract is published
// into the provider secret verbatim, the provider's own InstanceClass and MachineTemplate kinds are
// installed as CRDs, and a v1-era cluster (checksum-named template, MachineDeployment pointing at
// it) is recreated before node-controller is let near it.
//
// Only DVP can be verified on a real cloud; for the other six this is the closest thing to it.
type providerContract struct {
	name string
	// contractPath is the file the provider module ships.
	contractPath string
	// instanceClassKind and the spec below are what the user's InstanceClass looks like.
	instanceClassKind string
	instanceClass     map[string]any
	// providerConfig is this provider's subtree of the d8-node-manager-cloud-provider secret.
	providerConfig map[string]any
	// templateKind/templateAPIVersion mirror the provider's registration secret.
	templateKind       string
	templateAPIVersion string
	// rollingEdit changes a field the provider declared in rolloutFields.
	rollingEdit func(spec map[string]any)
}

func providerContracts() []providerContract {
	return []providerContract{
		{
			name:               "dvp",
			contractPath:       "../../../../../../../030-cloud-provider-dvp/capi/template.yaml",
			instanceClassKind:  "DVPInstanceClass",
			templateKind:       "DeckhouseMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
			providerConfig:     map[string]any{},
			instanceClass: map[string]any{
				"virtualMachine": map[string]any{
					"virtualMachineClassName": "generic",
					"cpu":                     map[string]any{"cores": int64(4)},
					"memory":                  map[string]any{"size": "8Gi"},
				},
				"rootDisk": map[string]any{
					"size":  "50Gi",
					"image": map[string]any{"kind": "ClusterVirtualImage", "name": "ubuntu"},
				},
			},
			rollingEdit: func(spec map[string]any) {
				spec["rootDisk"].(map[string]any)["size"] = "100Gi"
			},
		},
		{
			name:               "yandex",
			contractPath:       "../../../../../../../030-cloud-provider-yandex/capi/template.yaml",
			instanceClassKind:  "YandexInstanceClass",
			templateKind:       "YandexMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
			providerConfig: map[string]any{
				"instanceClassDefaults":       map[string]any{"imageID": "fd8default"},
				"zoneToSubnetIdMap":           map[string]any{"zone-a": "e9bsubnet"},
				"sshKey":                      "ssh-ed25519 AAAA",
				"nodeNetworkCIDR":             "10.222.0.0/16",
				"shouldAssignPublicIPAddress": false,
			},
			instanceClass: map[string]any{"cores": int64(4), "memory": int64(8192)},
			rollingEdit:   func(spec map[string]any) { spec["cores"] = int64(8) },
		},
		{
			name:               "openstack",
			contractPath:       "../../../../../../../../ee/modules/030-cloud-provider-openstack/capi/template.yaml",
			instanceClassKind:  "OpenStackInstanceClass",
			templateKind:       "OpenStackMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
			providerConfig: map[string]any{
				"instances":            map[string]any{"imageName": "ubuntu", "mainNetwork": "public"},
				"podNetworkMode":       "DirectRouting",
				"internalNetworkNames": []any{"internal"},
			},
			instanceClass: map[string]any{"flavorName": "m1.large"},
			rollingEdit:   func(spec map[string]any) { spec["flavorName"] = "m1.xlarge" },
		},
		{
			name:               "huaweicloud",
			contractPath:       "../../../../../../../../ee/modules/030-cloud-provider-huaweicloud/capi/template.yaml",
			instanceClassKind:  "HuaweiCloudInstanceClass",
			templateKind:       "HuaweiCloudMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
			providerConfig: map[string]any{
				"subnetId":        "subnet-default",
				"securityGroupId": "sg-default",
			},
			instanceClass: map[string]any{"flavorName": "s6.large.2", "imageName": "ubuntu"},
			rollingEdit:   func(spec map[string]any) { spec["flavorName"] = "s6.xlarge.2" },
		},
		{
			name:               "dynamix",
			contractPath:       "../../../../../../../../ee/modules/030-cloud-provider-dynamix/capi/template.yaml",
			instanceClassKind:  "DynamixInstanceClass",
			templateKind:       "DynamixMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1alpha1",
			providerConfig:     map[string]any{},
			instanceClass:      map[string]any{"imageName": "ubuntu", "numCPUs": int64(4), "memory": int64(8192)},
			rollingEdit:        func(spec map[string]any) { spec["numCPUs"] = int64(8) },
		},
		{
			name:               "vcd",
			contractPath:       "../../../../../../../../ee/modules/030-cloud-provider-vcd/capi/template.yaml",
			instanceClassKind:  "VCDInstanceClass",
			templateKind:       "VCDMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
			providerConfig:     map[string]any{"metadata": map[string]any{"owner": "platform"}},
			instanceClass:      map[string]any{"storageProfile": "vHDD", "template": "org/catalog/ubuntu"},
			rollingEdit:        func(spec map[string]any) { spec["storageProfile"] = "vSAN" },
		},
		{
			name:               "zvirt",
			contractPath:       "../../../../../../../../ee/se-plus/modules/030-cloud-provider-zvirt/capi/template.yaml",
			instanceClassKind:  "ZvirtInstanceClass",
			templateKind:       "ZvirtMachineTemplate",
			templateAPIVersion: "infrastructure.cluster.x-k8s.io/v1",
			providerConfig:     map[string]any{},
			instanceClass:      map[string]any{"template": "ubuntu", "numCPUs": int64(4), "memory": int64(8192)},
			rollingEdit:        func(spec map[string]any) { spec["numCPUs"] = int64(8) },
		},
	}
}

var _ = Describe("shipped provider contracts", Ordered, func() {
	const (
		zone       = "zone-a"
		eventually = 25 * time.Second
		poll       = 250 * time.Millisecond
	)

	// ensureCRD installs a schema-free CRD for a kind the provider owns in production. Schema-free
	// on purpose: what is under test is node-controller's handling of the provider's file, not the
	// provider's own validation.
	ensureCRD := func(apiVersion, kind string, clusterScoped bool) {
		group := strings.Split(apiVersion, "/")[0]
		version := strings.Split(apiVersion, "/")[1]
		// Kubernetes' own pluralization: a kind ending in "s" (…InstanceClass) takes "es".
		plural := strings.ToLower(kind) + "s"
		if strings.HasSuffix(strings.ToLower(kind), "s") {
			plural = strings.ToLower(kind) + "es"
		}

		scope := apiextensionsv1.NamespaceScoped
		if clusterScoped {
			scope = apiextensionsv1.ClusterScoped
		}

		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: plural + "." + group},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: group,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind: kind, ListKind: kind + "List", Plural: plural, Singular: strings.ToLower(kind),
				},
				Scope: scope,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
					Name: version, Served: true, Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: ptr(true),
						},
					},
				}},
			},
		}
		err := k8sClient.Create(suiteCtx, crd)
		if apierrors.IsAlreadyExists(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) bool {
			established := &apiextensionsv1.CustomResourceDefinition{}
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: crd.Name}, established)).To(Succeed())
			for _, condition := range established.Status.Conditions {
				if condition.Type == apiextensionsv1.Established {
					return condition.Status == apiextensionsv1.ConditionTrue
				}
			}
			return false
		}, eventually, poll).Should(BeTrue(), "the CRD must be established before objects of that kind are created")
	}

	// publishProvider points the whole cluster at one provider: its discovery secret and its real
	// contract file. The suite's own DVP-shaped fixture is restored afterwards.
	publishProvider := func(p providerContract) {
		contract, err := os.ReadFile(p.contractPath)
		Expect(err).NotTo(HaveOccurred(), "the provider must ship %s", p.contractPath)

		discovery := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName,
		}, discovery)).To(Succeed())
		discovery.Data["type"] = jsonBytes(p.name)
		discovery.Data["instanceClassKind"] = []byte(p.instanceClassKind)
		discovery.Data["capiMachineTemplateKind"] = []byte(p.templateKind)
		discovery.Data["capiMachineTemplateAPIVersion"] = []byte(p.templateAPIVersion)
		discovery.Data[p.name] = jsonBytes(p.providerConfig)
		Expect(k8sClient.Update(suiteCtx, discovery)).To(Succeed())

		// The suite already ships a DVP-shaped template secret, so this is create-or-update: a
		// plain Create would silently leave the v1 files in place and the legacy engine would run.
		templates := &corev1.Secret{}
		name := types.NamespacedName{
			Namespace: providerTemplateSecretNamespace,
			Name:      fmt.Sprintf("d8-cloud-provider-%s-capi", p.name),
		}
		err = k8sClient.Get(suiteCtx, name, templates)
		if apierrors.IsNotFound(err) {
			templates.Namespace, templates.Name = name.Namespace, name.Name
			templates.Data = map[string][]byte{machineTemplateContractKey: contract}
			Expect(k8sClient.Create(suiteCtx, templates)).To(Succeed())
			return
		}
		Expect(err).NotTo(HaveOccurred())
		if templates.Data == nil {
			templates.Data = map[string][]byte{}
		}
		templates.Data[machineTemplateContractKey] = contract
		Expect(k8sClient.Update(suiteCtx, templates)).To(Succeed())
	}

	// withdrawContract puts the provider secret back the way the rest of the suite expects it: the
	// v1 files only, so the legacy specs keep exercising the legacy engine.
	withdrawContract := func(p providerContract) {
		templates := &corev1.Secret{}
		name := types.NamespacedName{
			Namespace: providerTemplateSecretNamespace,
			Name:      fmt.Sprintf("d8-cloud-provider-%s-capi", p.name),
		}
		if err := k8sClient.Get(suiteCtx, name, templates); err != nil {
			return
		}
		delete(templates.Data, machineTemplateContractKey)
		Expect(k8sClient.Update(suiteCtx, templates)).To(Succeed())
	}

	restoreSuiteProvider := func(p providerContract) {
		discovery := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: cloudProviderSecretNamespace, Name: cloudProviderSecretName,
		}, discovery)).To(Succeed())
		discovery.Data["type"] = jsonBytes("dvp")
		discovery.Data["instanceClassKind"] = []byte("DVPInstanceClass")
		discovery.Data["capiMachineTemplateKind"] = []byte("DeckhouseMachineTemplate")
		discovery.Data["capiMachineTemplateAPIVersion"] = []byte("infrastructure.cluster.x-k8s.io/v1alpha1")
		delete(discovery.Data, p.name)
		Expect(k8sClient.Update(suiteCtx, discovery)).To(Succeed())
	}

	for _, provider := range providerContracts() {
		It(provider.name+": adopts a v1-era template, then rolls only on a declared field", func() {
			ensureCRD("deckhouse.io/v1alpha1", provider.instanceClassKind, true)
			ensureCRD(provider.templateAPIVersion, provider.templateKind, false)
			publishProvider(provider)
			DeferCleanup(func() {
				withdrawContract(provider)
				restoreSuiteProvider(provider)
			})

			ngName := testenv.UniqueName("contract-" + provider.name)
			icName := ngName + "-ic"
			legacyName := ngName + "-8ad9c341"

			By("the InstanceClass the user has")
			ic := &unstructured.Unstructured{}
			ic.SetAPIVersion("deckhouse.io/v1alpha1")
			ic.SetKind(provider.instanceClassKind)
			ic.SetName(icName)
			ic.Object["spec"] = provider.instanceClass
			Expect(k8sClient.Create(suiteCtx, ic)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, ic) })

			By("what the v1 engine left in the cluster: a checksum-named template and an MD on it")
			legacy := &unstructured.Unstructured{}
			legacy.SetAPIVersion(provider.templateAPIVersion)
			legacy.SetKind(provider.templateKind)
			legacy.SetName(legacyName)
			legacy.SetNamespace(common.MachineNamespace)
			legacy.SetLabels(map[string]string{"heritage": "deckhouse", "module": "node-manager", "node-group": ngName})
			Expect(unstructured.SetNestedMap(legacy.Object, map[string]any{"spec": map[string]any{}}, "spec", "template")).To(Succeed())
			Expect(k8sClient.Create(suiteCtx, legacy)).To(Succeed())

			md := &unstructured.Unstructured{}
			md.SetAPIVersion("cluster.x-k8s.io/v1beta2")
			md.SetKind("MachineDeployment")
			md.SetName(fmt.Sprintf("%s-%s", ngName, sha256Hash("11111111-2222-3333-4444-555555555555"+zone)))
			md.SetNamespace(common.MachineNamespace)
			md.SetLabels(map[string]string{"heritage": "deckhouse", "module": "node-manager", "node-group": ngName})
			Expect(unstructured.SetNestedMap(md.Object, map[string]any{
				"clusterName": "dvp",
				"replicas":    int64(1),
				"selector":    map[string]any{"matchLabels": map[string]any{"node-group": ngName}},
				"template": map[string]any{
					"metadata": map[string]any{"labels": map[string]any{"node-group": ngName}},
					"spec": map[string]any{
						"clusterName": "dvp",
						"bootstrap":   map[string]any{"dataSecretName": ngName + "-legacy"},
						"infrastructureRef": map[string]any{
							"apiGroup": strings.Split(provider.templateAPIVersion, "/")[0],
							"kind":     provider.templateKind,
							"name":     legacyName,
						},
					},
				},
			}, "spec")).To(Succeed())
			Expect(k8sClient.Create(suiteCtx, md)).To(Succeed())

			ng := &deckhousev1.NodeGroup{}
			ng.Name = ngName
			ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
			ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
				ClassReference: deckhousev1.ClassReference{Kind: provider.instanceClassKind, Name: icName},
				MinPerZone:     1,
				MaxPerZone:     1,
				Zones:          []string{zone},
			}
			Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(suiteCtx, ng)
				Eventually(func(g Gomega) int {
					list := &unstructured.UnstructuredList{}
					list.SetAPIVersion(provider.templateAPIVersion)
					list.SetKind(provider.templateKind + "List")
					g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
						client.MatchingLabels{"node-group": ngName})).To(Succeed())
					return len(list.Items)
				}, eventually, poll).Should(BeZero())
			})

			templateNames := func(g Gomega) []string {
				list := &unstructured.UnstructuredList{}
				list.SetAPIVersion(provider.templateAPIVersion)
				list.SetKind(provider.templateKind + "List")
				g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
					client.MatchingLabels{"node-group": ngName})).To(Succeed())
				names := make([]string, 0, len(list.Items))
				for _, item := range list.Items {
					names = append(names, item.GetName())
				}
				return names
			}

			referenced := func(g Gomega) string {
				live := &unstructured.Unstructured{}
				live.SetAPIVersion("cluster.x-k8s.io/v1beta2")
				live.SetKind("MachineDeployment")
				g.Expect(k8sClient.Get(suiteCtx, client.ObjectKeyFromObject(md), live)).To(Succeed())
				name, _, _ := unstructured.NestedString(live.Object, "spec", "template", "spec", "infrastructureRef", "name")
				return name
			}

			By("the upgrade adopts what is there: snapshot written, name kept, no second object")
			Eventually(func(g Gomega) {
				adopted := &unstructured.Unstructured{}
				adopted.SetAPIVersion(provider.templateAPIVersion)
				adopted.SetKind(provider.templateKind)
				g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
					Namespace: common.MachineNamespace, Name: legacyName,
				}, adopted)).To(Succeed())
				g.Expect(adopted.GetAnnotations()).To(HaveKey(machinetemplate.AppliedInstanceClassAnnotation))
			}, eventually, poll).Should(Succeed())

			Consistently(func(g Gomega) []string { return templateNames(g) },
				3*time.Second, poll).Should(ConsistOf(legacyName), "the upgrade must not create a generation")
			Expect(referenced(Default)).To(Equal(legacyName), "switching the reference is what rolls machines")

			By("a change to a declared rolloutField does create the next generation")
			live := &unstructured.Unstructured{}
			live.SetAPIVersion("deckhouse.io/v1alpha1")
			live.SetKind(provider.instanceClassKind)
			Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: icName}, live)).To(Succeed())
			spec, _, err := unstructured.NestedMap(live.Object, "spec")
			Expect(err).NotTo(HaveOccurred())
			provider.rollingEdit(spec)
			live.Object["spec"] = spec
			Expect(k8sClient.Update(suiteCtx, live)).To(Succeed())

			// In production node-controller watches the provider's InstanceClass kind, discovered
			// when the manager starts. Here the CRD is installed by this very spec, long after
			// that discovery, so the edit above wakes nothing — nudge the NodeGroup instead of
			// waiting out the resync.
			fresh := &deckhousev1.NodeGroup{}
			Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: ngName}, fresh)).To(Succeed())
			fresh.SetAnnotations(map[string]string{"test.deckhouse.io/nudge": "1"})
			Expect(k8sClient.Update(suiteCtx, fresh)).To(Succeed())

			Eventually(func(g Gomega) string { return referenced(g) },
				eventually, poll).Should(HaveSuffix("-gen1"))
		})
	}
})

func jsonBytes(value any) []byte {
	encoded, err := json.Marshal(value)
	Expect(err).NotTo(HaveOccurred())
	return encoded
}
