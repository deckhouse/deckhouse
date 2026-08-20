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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/cloudprovider"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/machinetemplate"
	"github.com/deckhouse/node-controller/internal/testenv"
)

// User story: As a cluster operator, I want a Deckhouse upgrade that switches my cloud provider to
// the v2 machine-template contract to change nothing about my running machines, and from then on I
// want machines to be recreated exactly when I change something that requires it — with the reason
// written on the NodeGroup.
//
// These specs are the enforcement of that promise. The v1 engine is still exercised by the rest of
// the suite; this file adds the v2 path by publishing the contract file into the same provider
// secret (its presence is the switch) and removing it again afterwards.
var _ = Describe("CAPI machine-template v2 contract", func() {
	const (
		clusterUUID = "11111111-2222-3333-4444-555555555555"
		zone        = "zone-a"
		eventually  = 20 * time.Second
		poll        = 250 * time.Millisecond
	)

	// The contract used by these specs. It is deliberately not a provider's real file: what is
	// under test is node-controller's generation machinery, not a provider's template.
	v2Contract := func(rolloutFields []string) string {
		fields := ""
		for _, field := range rolloutFields {
			fields += "  - " + field + "\n"
		}
		return `version: v2
rolloutFields:
` + fields + `machineDeployment:
  additionalFields:
    failureDomain: "{{ .zone }}"
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
  kind: DeckhouseMachineTemplate
  spec:
    template:
      spec:
        vmClassName: {{ .instanceClass.vmClassName }}
        {{- if get .instanceClass "rootDiskSize" }}
        rootDiskSize: {{ .instanceClass.rootDiskSize | quote }}
        {{- end }}
`
	}

	publishContract := func(contract string) {
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: providerTemplateSecretNamespace, Name: "d8-cloud-provider-dvp-capi",
		}, secret)).To(Succeed())
		secret.Data[machineTemplateContractKey] = []byte(contract)
		Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
	}

	withdrawContract := func() {
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
			Namespace: providerTemplateSecretNamespace, Name: "d8-cloud-provider-dvp-capi",
		}, secret)).To(Succeed())
		delete(secret.Data, machineTemplateContractKey)
		Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
	}

	newTemplateObject := func(name string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha1")
		obj.SetKind("DeckhouseMachineTemplate")
		if name != "" {
			obj.SetName(name)
			obj.SetNamespace(common.MachineNamespace)
		}
		return obj
	}

	newInstanceClass := func(name string, spec map[string]any) *unstructured.Unstructured {
		ic := &unstructured.Unstructured{}
		ic.SetAPIVersion("deckhouse.io/v1alpha1")
		ic.SetKind("DVPInstanceClass")
		ic.SetName(name)
		ic.Object["spec"] = spec
		return ic
	}

	newNodeGroup := func(name, icName string) *deckhousev1.NodeGroup {
		ng := &deckhousev1.NodeGroup{}
		ng.Name = name
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
			ClassReference: deckhousev1.ClassReference{Kind: "DVPInstanceClass", Name: icName},
			MinPerZone:     1,
			MaxPerZone:     1,
			Zones:          []string{zone},
		}
		return ng
	}

	machineTemplates := func(g Gomega, ngName string) []unstructured.Unstructured {
		list := &unstructured.UnstructuredList{}
		list.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1alpha1")
		list.SetKind("DeckhouseMachineTemplateList")
		g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
			client.MatchingLabels{"node-group": ngName})).To(Succeed())
		return list.Items
	}

	machineDeployment := func(g Gomega, ngName string) *capiv1beta2.MachineDeployment {
		list := &capiv1beta2.MachineDeploymentList{}
		g.Expect(k8sClient.List(suiteCtx, list, client.InNamespace(common.MachineNamespace),
			client.MatchingLabels{"node-group": ngName})).To(Succeed())
		g.Expect(list.Items).To(HaveLen(1))
		return &list.Items[0]
	}

	// referencedTemplateName is the single source of truth for "which generation is current" —
	// exactly what node-controller itself reads.
	referencedTemplateName := func(g Gomega, ngName string) string {
		return machineDeployment(g, ngName).Spec.Template.Spec.InfrastructureRef.Name
	}

	// nudge triggers a reconcile of the NodeGroup. Needed after changing the provider contract:
	// node-controller watches the cloud-provider discovery secret, not the per-provider template
	// secret, so a contract edit alone would wait for the resync.
	setAnnotation := func(ng *deckhousev1.NodeGroup, key, value string) {
		fresh := &deckhousev1.NodeGroup{}
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: ng.Name}, fresh)).To(Succeed())
		annotations := fresh.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[key] = value
		fresh.SetAnnotations(annotations)
		Expect(k8sClient.Update(suiteCtx, fresh)).To(Succeed())
	}

	nudges := 0
	nudge := func(ng *deckhousev1.NodeGroup) {
		nudges++
		setAnnotation(ng, "test.deckhouse.io/nudge", fmt.Sprintf("%d", nudges))
	}

	updateInstanceClass := func(name string, spec map[string]any) {
		ic := newInstanceClass(name, nil)
		Expect(k8sClient.Get(suiteCtx, types.NamespacedName{Name: name}, ic)).To(Succeed())
		ic.Object["spec"] = spec
		Expect(k8sClient.Update(suiteCtx, ic)).To(Succeed())
	}

	// setUp publishes the contract, the InstanceClass and the NodeGroup, and waits until the
	// first generation exists. It returns the NodeGroup and its InstanceClass name.
	setUp := func(namePrefix string, rolloutFields []string, icSpec map[string]any) (*deckhousev1.NodeGroup, string) {
		publishContract(v2Contract(rolloutFields))
		DeferCleanup(withdrawContract)

		icName := testenv.UniqueName(namePrefix + "-ic")
		Expect(k8sClient.Create(suiteCtx, newInstanceClass(icName, icSpec))).To(Succeed())
		DeferCleanup(func() {
			ic := newInstanceClass(icName, icSpec)
			_ = k8sClient.Delete(suiteCtx, ic)
		})

		ng := newNodeGroup(testenv.UniqueName(namePrefix), icName)
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(suiteCtx, ng)
			Eventually(func(g Gomega) int { return len(machineTemplates(g, ng.Name)) },
				eventually, poll).Should(BeZero())
		})
		return ng, icName
	}

	It("names the first generation after the zone and gen1, and records the snapshot", func() {
		ng, _ := setUp("v2-fresh", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		var template unstructured.Unstructured
		Eventually(func(g Gomega) {
			templates := machineTemplates(g, ng.Name)
			g.Expect(templates).To(HaveLen(1))
			template = templates[0]
			g.Expect(referencedTemplateName(g, ng.Name)).To(Equal(template.GetName()))
		}, eventually, poll).Should(Succeed())

		Expect(template.GetName()).To(Equal(fmt.Sprintf("%s-%s-gen1", ng.Name, sha256Hash(clusterUUID+zone))))

		By("the snapshot holds the whole InstanceClass spec, not just the rolloutFields")
		snapshot := map[string]any{}
		Expect(json.Unmarshal([]byte(template.GetAnnotations()[machinetemplate.AppliedInstanceClassAnnotation]), &snapshot)).To(Succeed())
		Expect(snapshot).To(Equal(map[string]any{"vmClassName": "generic"}))
		Expect(template.GetAnnotations()).To(HaveKey(machinetemplate.AppliedRolloutIDAnnotation))

		By("the template renders no metadata of its own: node-controller owns it")
		Expect(template.GetLabels()).To(HaveKeyWithValue("node-group", ng.Name))
		Expect(template.GetLabels()).To(HaveKeyWithValue("heritage", "deckhouse"))

		By("machineDeployment.additionalFields reached the MachineDeployment")
		Eventually(func(g Gomega) string {
			return machineDeployment(g, ng.Name).Spec.Template.Spec.FailureDomain
		}, eventually, poll).Should(Equal(zone))
	})

	// The cloud-provider config is the second input the template renders from, and for one provider
	// (vcd) it is also part of the rollout decision — its VCDClusterConfiguration schema promises
	// the user that changing `metadata` recreates CloudEphemeral nodes. A contract that can only
	// name InstanceClass fields turns that promise into silence: the template keeps rendering the
	// new value, so machines created later disagree with the ones already running.
	Describe("the provider-config axis", func() {
		v2ProviderContract := func(rolloutFields, providerRolloutFields []string) string {
			list := func(fields []string) string {
				out := ""
				for _, field := range fields {
					out += "  - " + field + "\n"
				}
				return out
			}
			return `version: v2
rolloutFields:
` + list(rolloutFields) + `providerRolloutFields:
` + list(providerRolloutFields) + `template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
  kind: DeckhouseMachineTemplate
  spec:
    template:
      spec:
        vmClassName: {{ .instanceClass.vmClassName }}
        {{- if get .provider "datacenter" }}
        datacenter: {{ .provider.datacenter | quote }}
        {{- end }}
`
		}

		setProviderConfig := func(config map[string]any) {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: cloudprovider.RegistrationSecretNamespace, Name: cloudprovider.RegistrationSecretNamePrefix,
			}, secret)).To(Succeed())
			raw, err := json.Marshal(config)
			Expect(err).NotTo(HaveOccurred())
			secret.Data["dvp"] = raw
			Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
		}

		clearProviderConfig := func() {
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: cloudprovider.RegistrationSecretNamespace, Name: cloudprovider.RegistrationSecretNamePrefix,
			}, secret)).To(Succeed())
			delete(secret.Data, "dvp")
			Expect(k8sClient.Update(suiteCtx, secret)).To(Succeed())
		}

		// setUpWithProvider mirrors setUp, with the provider config published before the contract.
		setUpWithProvider := func(namePrefix string, contract string, config map[string]any) *deckhousev1.NodeGroup {
			setProviderConfig(config)
			DeferCleanup(clearProviderConfig)

			publishContract(contract)
			DeferCleanup(withdrawContract)

			icName := testenv.UniqueName(namePrefix + "-ic")
			Expect(k8sClient.Create(suiteCtx, newInstanceClass(icName, map[string]any{"vmClassName": "generic"}))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, newInstanceClass(icName, nil)) })

			ng := newNodeGroup(testenv.UniqueName(namePrefix), icName)
			Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(suiteCtx, ng)
				Eventually(func(g Gomega) int { return len(machineTemplates(g, ng.Name)) },
					eventually, poll).Should(BeZero())
			})

			Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
				eventually, poll).Should(HaveSuffix("-gen1"))
			return ng
		}

		It("creates a new generation when a declared provider-config field changes", func() {
			ng := setUpWithProvider("v2-provider-roll",
				v2ProviderContract([]string{"vmClassName"}, []string{"datacenter"}),
				map[string]any{"datacenter": "dc-1"})

			By("the snapshot records the provider config the object was rendered from")
			Eventually(func(g Gomega) map[string]any {
				templates := machineTemplates(g, ng.Name)
				g.Expect(templates).To(HaveLen(1))
				snapshot := map[string]any{}
				g.Expect(json.Unmarshal(
					[]byte(templates[0].GetAnnotations()[machinetemplate.AppliedProviderConfigAnnotation]),
					&snapshot)).To(Succeed())
				return snapshot
			}, eventually, poll).Should(Equal(map[string]any{"datacenter": "dc-1"}))

			setProviderConfig(map[string]any{"datacenter": "dc-2"})
			nudge(ng)

			Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
				eventually, poll).Should(HaveSuffix("-gen2"))

			By("the reason on the NodeGroup names the provider config, not the InstanceClass")
			Eventually(func(g Gomega) string {
				events := &corev1.EventList{}
				g.Expect(k8sClient.List(suiteCtx, events, client.InNamespace(""))).To(Succeed())
				for _, event := range events.Items {
					if event.InvolvedObject.Name == ng.Name && event.Reason == "MachinesRollout" {
						return event.Message
					}
				}
				return ""
			}, eventually, poll).Should(ContainSubstring(`providerConfig datacenter "dc-1" → "dc-2"`))
		})

		// The provider subtree of d8-node-manager-cloud-provider carries the cloud credentials:
		// vcd's password and apiToken, yandex's serviceAccountJSON, huaweicloud's accessKey and
		// secretKey, openstack's connection.password. The MachineTemplate is read far more widely
		// than that Secret, so only the fields the provider declared may be recorded on it.
		// The migration promise, restated for this axis: the adoption spec above runs on a contract
		// with no providerRolloutFields, so it cannot show what a vcd-shaped cluster does. Adoption
		// must record the declared provider fields, or the very next reconcile would compare the
		// current config against an empty snapshot and roll every machine in the cluster.
		It("adopts a v1-era template without rolling when providerRolloutFields are declared", func() {
			setProviderConfig(map[string]any{"datacenter": "dc-1", "password": "s3cret"})
			DeferCleanup(clearProviderConfig)

			publishContract(v2ProviderContract([]string{"vmClassName"}, []string{"datacenter"}))
			DeferCleanup(withdrawContract)

			icName := testenv.UniqueName("v2-provider-adopt-ic")
			Expect(k8sClient.Create(suiteCtx, newInstanceClass(icName, map[string]any{"vmClassName": "generic"}))).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(suiteCtx, newInstanceClass(icName, nil)) })

			ngName := testenv.UniqueName("v2-provider-adopt")
			legacyName := ngName + "-8ad9c341"

			legacy := newTemplateObject(legacyName)
			legacy.SetLabels(map[string]string{"heritage": "deckhouse", "module": "node-manager", "node-group": ngName})
			Expect(unstructured.SetNestedField(legacy.Object, "generic", "spec", "template", "spec", "vmClassName")).To(Succeed())
			Expect(k8sClient.Create(suiteCtx, legacy)).To(Succeed())

			md := &unstructured.Unstructured{}
			md.SetAPIVersion("cluster.x-k8s.io/v1beta2")
			md.SetKind("MachineDeployment")
			md.SetName(fmt.Sprintf("%s-%s", ngName, sha256Hash(clusterUUID+zone)))
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
						"bootstrap":   map[string]any{"dataSecretName": ngName + "-legacy-bootstrap"},
						"infrastructureRef": map[string]any{
							"apiGroup": "infrastructure.cluster.x-k8s.io",
							"kind":     "DeckhouseMachineTemplate",
							"name":     legacyName,
						},
					},
				},
			}, "spec")).To(Succeed())
			Expect(k8sClient.Create(suiteCtx, md)).To(Succeed())

			ng := newNodeGroup(ngName, icName)
			Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(suiteCtx, ng)
				Eventually(func(g Gomega) int { return len(machineTemplates(g, ng.Name)) },
					eventually, poll).Should(BeZero())
			})

			By("the adopted object records the declared provider field, and only it")
			Eventually(func(g Gomega) map[string]any {
				adopted := newTemplateObject(legacyName)
				g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
					Name: legacyName, Namespace: common.MachineNamespace,
				}, adopted)).To(Succeed())
				raw, ok := adopted.GetAnnotations()[machinetemplate.AppliedProviderConfigAnnotation]
				g.Expect(ok).To(BeTrue())
				snapshot := map[string]any{}
				g.Expect(json.Unmarshal([]byte(raw), &snapshot)).To(Succeed())
				return snapshot
			}, eventually, poll).Should(Equal(map[string]any{"datacenter": "dc-1"}))

			By("the MachineDeployment still points at the v1 name: no generation, no rollout")
			Consistently(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
				5*time.Second, poll).Should(Equal(legacyName))
		})

		It("keeps provider credentials out of the snapshot annotation", func() {
			ng := setUpWithProvider("v2-provider-secret",
				v2ProviderContract([]string{"vmClassName"}, []string{"datacenter"}),
				map[string]any{"datacenter": "dc-1", "password": "s3cret"})

			Eventually(func(g Gomega) map[string]any {
				templates := machineTemplates(g, ng.Name)
				g.Expect(templates).To(HaveLen(1))
				snapshot := map[string]any{}
				g.Expect(json.Unmarshal(
					[]byte(templates[0].GetAnnotations()[machinetemplate.AppliedProviderConfigAnnotation]),
					&snapshot)).To(Succeed())
				return snapshot
			}, eventually, poll).Should(Equal(map[string]any{"datacenter": "dc-1"}))
		})

		It("leaves the generation alone when an undeclared provider-config field changes", func() {
			ng := setUpWithProvider("v2-provider-quiet",
				v2ProviderContract([]string{"vmClassName"}, []string{"datacenter"}),
				map[string]any{"datacenter": "dc-1", "sshKey": "ssh-ed25519 AAAA"})

			current := ""
			Eventually(func(g Gomega) string {
				current = referencedTemplateName(g, ng.Name)
				return current
			}, eventually, poll).Should(HaveSuffix("-gen1"))

			setProviderConfig(map[string]any{"datacenter": "dc-1", "sshKey": "ssh-ed25519 BBBB"})
			nudge(ng)

			Consistently(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
				5*time.Second, poll).Should(Equal(current))
		})
	})

	// The migration promise: switching a live cluster to v2 must not touch a single machine.
	It("adopts a checksum-named template from the v1 era instead of creating a generation", func() {
		publishContract(v2Contract([]string{"vmClassName"}))
		DeferCleanup(withdrawContract)

		icName := testenv.UniqueName("v2-adopt-ic")
		Expect(k8sClient.Create(suiteCtx, newInstanceClass(icName, map[string]any{"vmClassName": "generic"}))).To(Succeed())

		ngName := testenv.UniqueName("v2-adopt")
		legacyName := ngName + "-8ad9c341"

		By("recreating what the v1 engine leaves in a cluster: a checksum-named template and an MD pointing at it")
		legacy := newTemplateObject(legacyName)
		legacy.SetLabels(map[string]string{"heritage": "deckhouse", "module": "node-manager", "node-group": ngName})
		Expect(unstructured.SetNestedField(legacy.Object, "generic", "spec", "template", "spec", "vmClassName")).To(Succeed())
		Expect(k8sClient.Create(suiteCtx, legacy)).To(Succeed())

		md := &unstructured.Unstructured{}
		md.SetAPIVersion("cluster.x-k8s.io/v1beta2")
		md.SetKind("MachineDeployment")
		md.SetName(fmt.Sprintf("%s-%s", ngName, sha256Hash(clusterUUID+zone)))
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
					"bootstrap":   map[string]any{"dataSecretName": ngName + "-legacy-bootstrap"},
					"infrastructureRef": map[string]any{
						"apiGroup": "infrastructure.cluster.x-k8s.io",
						"kind":     "DeckhouseMachineTemplate",
						"name":     legacyName,
					},
				},
			},
		}, "spec")).To(Succeed())
		Expect(k8sClient.Create(suiteCtx, md)).To(Succeed())

		ng := newNodeGroup(ngName, icName)
		Expect(k8sClient.Create(suiteCtx, ng)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(suiteCtx, ng)
			Eventually(func(g Gomega) int { return len(machineTemplates(g, ngName)) }, eventually, poll).Should(BeZero())
		})

		By("the template is adopted: same object, snapshot written, no new generation")
		Eventually(func(g Gomega) {
			adopted := newTemplateObject("")
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: common.MachineNamespace, Name: legacyName,
			}, adopted)).To(Succeed())
			g.Expect(adopted.GetAnnotations()).To(HaveKey(machinetemplate.AppliedInstanceClassAnnotation))
			g.Expect(adopted.GetUID()).To(Equal(legacy.GetUID()), "adoption must not recreate the object")
		}, eventually, poll).Should(Succeed())

		Consistently(func(g Gomega) []string {
			names := []string{}
			for _, template := range machineTemplates(g, ngName) {
				names = append(names, template.GetName())
			}
			return names
		}, 3*time.Second, poll).Should(ConsistOf(legacyName), "no second generation may appear")

		Expect(referencedTemplateName(Default, ngName)).To(Equal(legacyName),
			"the MachineDeployment must keep pointing at the adopted template — switching it rolls every machine")
	})

	It("does not create a generation when a field outside rolloutFields changes", func() {
		ng, icName := setUp("v2-noroll", []string{"vmClassName"},
			map[string]any{"vmClassName": "generic", "capacity": map[string]any{"cores": int64(4)}})

		var first string
		Eventually(func(g Gomega) string {
			first = referencedTemplateName(g, ng.Name)
			return first
		}, eventually, poll).ShouldNot(BeEmpty())

		updateInstanceClass(icName, map[string]any{"vmClassName": "generic", "capacity": map[string]any{"cores": int64(8)}})

		Consistently(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			5*time.Second, poll).Should(Equal(first))
	})

	It("creates the next generation when a rolloutField changes and says why on the NodeGroup", func() {
		ng, icName := setUp("v2-roll", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		var first string
		Eventually(func(g Gomega) string {
			first = referencedTemplateName(g, ng.Name)
			return first
		}, eventually, poll).Should(HaveSuffix("-gen1"))

		updateInstanceClass(icName, map[string]any{"vmClassName": "bigger"})

		Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			eventually, poll).Should(HaveSuffix("-gen2"))

		By("the previous generation is still there until the rollout finishes")
		Eventually(func(g Gomega) int { return len(machineTemplates(g, ng.Name)) },
			eventually, poll).Should(Equal(2))

		By("the operator can read the reason on the NodeGroup")
		Eventually(func(g Gomega) string {
			events := &corev1.EventList{}
			g.Expect(k8sClient.List(suiteCtx, events, client.InNamespace(""))).To(Succeed())
			for _, event := range events.Items {
				if event.InvolvedObject.Name == ng.Name && event.Reason == "MachinesRollout" {
					return event.Message
				}
			}
			return ""
		}, eventually, poll).Should(ContainSubstring(`vmClassName "generic" → "bigger"`))
	})

	It("creates the next generation when the operator sets manual-rollout-id", func() {
		ng, _ := setUp("v2-manual", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			eventually, poll).Should(HaveSuffix("-gen1"))

		setAnnotation(ng, "manual-rollout-id", "2026-07-31")

		Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			eventually, poll).Should(HaveSuffix("-gen2"))
	})

	// The snapshot stores facts (the whole spec), rolloutFields is policy applied at comparison
	// time — so a provider release that changes the list cannot roll anybody's machines.
	It("does not create a generation when the provider changes rolloutFields", func() {
		ng, _ := setUp("v2-fields", []string{"vmClassName", "rootDiskSize"},
			map[string]any{"vmClassName": "generic", "rootDiskSize": "50Gi"})

		var first string
		Eventually(func(g Gomega) string {
			first = referencedTemplateName(g, ng.Name)
			return first
		}, eventually, poll).Should(HaveSuffix("-gen1"))

		publishContract(v2Contract([]string{"vmClassName"}))
		nudge(ng)

		Consistently(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			5*time.Second, poll).Should(Equal(first))
	})

	// A CAPI infrastructure template is read afresh on every Machine creation, so editing one in
	// place does not roll machines — it silently changes what the old MachineSet builds from.
	It("never rewrites the spec of an existing generation when the template text changes", func() {
		ng, _ := setUp("v2-frozen", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		var name string
		Eventually(func(g Gomega) string {
			name = referencedTemplateName(g, ng.Name)
			return name
		}, eventually, poll).Should(HaveSuffix("-gen1"))

		publishContract(`version: v2
rolloutFields:
  - vmClassName
template: |
  apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
  kind: DeckhouseMachineTemplate
  spec:
    template:
      spec:
        vmClassName: {{ .instanceClass.vmClassName }}
        rootDiskSize: "999Gi"
`)
		nudge(ng)

		Consistently(func(g Gomega) map[string]any {
			template := newTemplateObject("")
			g.Expect(k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: common.MachineNamespace, Name: name,
			}, template)).To(Succeed())
			spec, _, err := unstructured.NestedMap(template.Object, "spec", "template", "spec")
			g.Expect(err).NotTo(HaveOccurred())
			return spec
		}, 5*time.Second, poll).Should(Equal(map[string]any{"vmClassName": "generic"}))
	})

	// The snapshot on a superseded generation is the only durable record of what a rollout changed
	// once the NodeGroup event has expired, so the pruner keeps a few of them instead of deleting
	// each one the moment CAPI retires its MachineSet.
	It("keeps the last few superseded generations and prunes the ones behind them", func() {
		ng, icName := setUp("v2-history", []string{"vmClassName"}, map[string]any{"vmClassName": "gen1"})

		Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			eventually, poll).Should(HaveSuffix("-gen1"))

		for _, class := range []string{"gen2", "gen3", "gen4"} {
			updateInstanceClass(icName, map[string]any{"vmClassName": class})
			Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
				eventually, poll).Should(HaveSuffix("-" + class))
		}

		Eventually(func(g Gomega) []string {
			names := []string{}
			for _, template := range machineTemplates(g, ng.Name) {
				names = append(names, template.GetName())
			}
			return names
		}, eventually, poll).Should(HaveLen(keptGenerations),
			"the current generation plus the ones kept for their snapshots, nothing older")

		By("and what survives is the newest, with its diff still readable")
		Eventually(func(g Gomega) []string {
			suffixes := []string{}
			for _, template := range machineTemplates(g, ng.Name) {
				name := template.GetName()
				suffixes = append(suffixes, name[strings.LastIndex(name, "-gen"):])
				g.Expect(template.GetAnnotations()).To(HaveKey(machinetemplate.AppliedInstanceClassAnnotation))
			}
			return suffixes
		}, eventually, poll).Should(ConsistOf("-gen2", "-gen3", "-gen4"))
	})

	// Rolling a Deckhouse release back to a version without the v2 engine is a rollout: the v1
	// path names templates by the instance-class checksum and knows nothing about generations, so
	// it creates its own object and switches the MachineDeployment to it. This spec pins that
	// consequence rather than leaving it to be discovered on a live cluster — it is the price of a
	// downgrade, it happens once, and the forward migration (adoption) is what must not roll.
	It("switches back to a checksum-named template when the provider contract is withdrawn", func() {
		ng, _ := setUp("v2-rollback", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		var generation string
		Eventually(func(g Gomega) string {
			generation = referencedTemplateName(g, ng.Name)
			return generation
		}, eventually, poll).Should(HaveSuffix("-gen1"))

		withdrawContract()
		nudge(ng)

		Eventually(func(g Gomega) string { return referencedTemplateName(g, ng.Name) },
			eventually, poll).ShouldNot(Equal(generation))
	})

	// If the object a live MachineDeployment references disappears, recreating it under a new
	// name would roll every machine of the group for nothing.
	It("recreates a deleted generation under the same name", func() {
		ng, _ := setUp("v2-restore", []string{"vmClassName"}, map[string]any{"vmClassName": "generic"})

		var name string
		Eventually(func(g Gomega) string {
			name = referencedTemplateName(g, ng.Name)
			return name
		}, eventually, poll).Should(HaveSuffix("-gen1"))

		Expect(k8sClient.Delete(suiteCtx, newTemplateObject(name))).To(Succeed())

		nudge(ng)

		Eventually(func(g Gomega) error {
			restored := newTemplateObject("")
			return k8sClient.Get(suiteCtx, types.NamespacedName{
				Namespace: common.MachineNamespace, Name: name,
			}, restored)
		}, eventually, poll).Should(Succeed())

		Expect(referencedTemplateName(Default, ng.Name)).To(Equal(name))
	})
})
