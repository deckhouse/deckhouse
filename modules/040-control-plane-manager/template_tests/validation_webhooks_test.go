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

package template_tests

import (
	"encoding/json"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

// The ValidationWebhooks of the module and the files of webhooks/validating their handler is made of,
// in the order the chart joins them; nil for a handler written inline in the manifest.
var controlPlaneManagerValidationWebhooks = map[string][]string{
	"control-plane-node-maintenance":         nil,
	"d8-secret-encryption-key-configuration": {"webhooks/validating/d8_secret_encryption_key_configuration.py"},
	"cpm-moduleconfig-feature-gates": {
		"webhooks/validating/feature_gates_generated.py",
		"webhooks/validating/k8s_version_common.py",
		"webhooks/validating/feature_gates.py",
	},
	"cpm-k8s-version-feature-gates": {
		"webhooks/validating/feature_gates_generated.py",
		"webhooks/validating/k8s_version_common.py",
		"webhooks/validating/k8s_version_feature_gates.py",
	},
	"cpm-k8s-version-feature-gates-mc": {
		"webhooks/validating/feature_gates_generated.py",
		"webhooks/validating/k8s_version_common.py",
		"webhooks/validating/k8s_version_feature_gates.py",
	},
}

// The snapshot bindings of the feature gate webhooks: the bindings of the module hooks they replaced,
// with the jqFilter that returns the fields the handlers read. webhook-operator sets includeSnapshotsFrom
// of a webhook to its context, so each context is the includeSnapshotsFrom of the module hook binding.
const (
	clusterConfigurationBinding = `{
		"name": "d8-cluster-configuration",
		"kubernetes": {
			"apiVersion": "v1",
			"kind": "Secret",
			"namespace": {"nameSelector": {"matchNames": ["kube-system"]}},
			"nameSelector": {"matchNames": ["d8-cluster-configuration"]},
			"jqFilter": "{\"data\": .data}"
		}
	}`
	clusterKubernetesBinding = `{
		"name": "d8-cluster-kubernetes",
		"kubernetes": {
			"apiVersion": "v1",
			"kind": "ConfigMap",
			"namespace": {"nameSelector": {"matchNames": ["kube-system"]}},
			"nameSelector": {"matchNames": ["d8-cluster-kubernetes"]},
			"jqFilter": "{\"data\": {\"spec\": .data.spec}}"
		}
	}`
	moduleConfigBinding = `{
		"name": "module-config-control-plane-manager",
		"kubernetes": {
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind": "ModuleConfig",
			"nameSelector": {"matchNames": ["control-plane-manager"]},
			"jqFilter": "{\"spec\": {\"settings\": .spec.settings}}"
		}
	}`
	moduleConfigRules = `[{
		"apiGroups": ["deckhouse.io"],
		"apiVersions": ["*"],
		"operations": ["CREATE", "UPDATE"],
		"resources": ["moduleconfigs"],
		"scope": "Cluster"
	}]`
	onlyControlPlaneManager = `[{
		"name": "only-control-plane-manager-module",
		"expression": "request.name == \"control-plane-manager\""
	}]`
)

var _ = Describe("Module :: control-plane-manager :: helm template :: validation webhooks", func() {
	f := SetupHelmConfig(`controlPlaneManager: {}`)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	resource := func(name string) object_store.KubeObject {
		object := f.KubernetesGlobalResource("ValidationWebhook", name)
		Expect(object.Exists()).To(BeTrue(), name)
		return object
	}

	BeforeEach(func() {
		// The values the publish api tests render the chart with.
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.discovery.kubernetesVersion", "1.33.13")
		f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		f.ValuesSet("global.modules.ingressClass", "nginx")
		f.ValuesSet("global.modules.https.mode", "CertManager")
		f.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		f.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		f.ValuesSet("global.enabledModules", []string{"cert-manager"})
		f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		f.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		f.ValuesSet("global.discovery.clusterDomain", "cluster.local")
		f.ValuesSet("controlPlaneManager.internal.effectiveKubernetesVersion", "1.33")
		f.ValuesSet("controlPlaneManager.internal.maxUsedKubernetesVersion", "1.33")
		f.ValuesSet("controlPlaneManager.internal.authn.enableBasicAuth", true)
		f.ValuesSet("controlPlaneManager.internal.authn.publishedAPIKubeconfigGeneratorMasterCA", "publishedapica")
		f.ValuesSet("controlPlaneManager.internal.selfSignedCA.cert", "test")
		f.ValuesSet("controlPlaneManager.internal.selfSignedCA.key", "testCA")
		f.ValuesSetFromYaml("controlPlaneManager.apiserver", publishAPIValues)
		f.ValuesSetFromYaml("controlPlaneManager.internal.resourcesRequests.components", `
kubeApiserver:
  milliCPU: 460
  memoryBytes: "241591910"
etcd:
  milliCPU: 358
  memoryBytes: "187904819"
kubeControllerManager:
  milliCPU: 102
  memoryBytes: "53687091"
kubeScheduler:
  milliCPU: 102
  memoryBytes: "53687091"
`)
		f.HelmRender()
	})

	It("Should check every ValidationWebhook the module declares", func() {
		declared, err := validationwebhook.ManifestNames(moduleDir)
		Expect(err).ShouldNot(HaveOccurred())
		checked := make([]string, 0, len(controlPlaneManagerValidationWebhooks))
		for name := range controlPlaneManagerValidationWebhooks {
			checked = append(checked, name)
		}
		Expect(checked).To(ConsistOf(declared))
	})

	It("Should render the handlers from webhooks/validating", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, files := range controlPlaneManagerValidationWebhooks {
			if files == nil {
				continue
			}
			handler := resource(name).Field("handler.python").String()
			Expect(validationwebhook.HandlerMatchesFiles(moduleDir, handler, files...)).To(Succeed(), name)
		}
	})

	It("Should keep the binding of d8-secret-encryption-key", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		object := resource("d8-secret-encryption-key-configuration")
		Expect(object.Field("validationObject").String()).To(MatchJSON(`{
			"name": "d8-secret-encryption-key-secret.deckhouse.io",
			"group": "main",
			"labelSelector": {"matchLabels": {"name": "d8-secret-encryption-key"}},
			"rules": [{
				"apiGroups": [""],
				"apiVersions": ["v1"],
				"operations": ["*"],
				"resources": ["secrets"],
				"scope": "Namespaced"
			}]
		}`))
		Expect(object.Field("context").Exists()).To(BeFalse())
	})

	It("Should keep the bindings of the feature gate webhooks", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		object := resource("cpm-moduleconfig-feature-gates")
		Expect(object.Field("validationObject").String()).To(MatchJSON(`{
			"name": "cpm-moduleconfig-feature-gates.deckhouse.io",
			"group": "main",
			"matchConditions": ` + onlyControlPlaneManager + `,
			"rules": ` + moduleConfigRules + `
		}`))
		Expect(object.Field("context").String()).To(MatchJSON(
			"[" + clusterConfigurationBinding + "," + clusterKubernetesBinding + "]"))

		object = resource("cpm-k8s-version-feature-gates")
		Expect(object.Field("validationObject").String()).To(MatchJSON(`{
			"name": "cpm-k8s-version-feature-gates.deckhouse.io",
			"group": "cpm-feature-gates-validation",
			"namespace": {"labelSelector": {"matchLabels": {"kubernetes.io/metadata.name": "kube-system"}}},
			"labelSelector": {"matchLabels": {"name": "d8-cluster-configuration"}},
			"rules": [{
				"apiGroups": [""],
				"apiVersions": ["v1"],
				"operations": ["UPDATE"],
				"resources": ["secrets"],
				"scope": "Namespaced"
			}]
		}`))
		// The handler takes the new cluster configuration from the request, not from a snapshot.
		Expect(object.Field("context").String()).To(MatchJSON(
			"[" + moduleConfigBinding + "," + clusterKubernetesBinding + "]"))

		object = resource("cpm-k8s-version-feature-gates-mc")
		Expect(object.Field("validationObject").String()).To(MatchJSON(`{
			"name": "cpm-k8s-version-feature-gates-mc.deckhouse.io",
			"group": "cpm-feature-gates-validation",
			"matchConditions": ` + onlyControlPlaneManager + `,
			"rules": ` + moduleConfigRules + `
		}`))
		Expect(object.Field("context").String()).To(MatchJSON(
			"[" + clusterConfigurationBinding + "," + clusterKubernetesBinding + "]"))
	})

	It("Should name the snapshots as the feature gate handlers read them", func() {
		constant := func(file, name string) string {
			value, err := validationwebhook.PythonString(moduleDir, "webhooks/validating/"+file, name)
			Expect(err).ShouldNot(HaveOccurred())
			return value
		}
		Expect(constant("k8s_version_common.py", "CLUSTER_CONFIG_SNAPSHOT_NAME")).To(Equal("d8-cluster-configuration"))
		Expect(constant("k8s_version_common.py", "CLUSTER_KUBERNETES_SNAPSHOT_NAME")).To(Equal("d8-cluster-kubernetes"))
		Expect(constant("k8s_version_feature_gates.py", "MODULE_CONFIG_SNAPSHOT_NAME")).To(Equal("module-config-control-plane-manager"))
	})

	It("Should give the feature gate handlers the fields they read", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		// The python tests hand the handlers snapshot items of exactly these shapes.
		jq := func(webhook, binding string, object string) interface{} {
			for _, item := range resource(webhook).Field("context").Array() {
				if item.Get("name").String() != binding {
					continue
				}
				result, err := validationwebhook.RunJQ(item.Get("kubernetes.jqFilter").String(), parseJSON(object))
				Expect(err).ShouldNot(HaveOccurred())
				return result
			}
			Fail(webhook + " has no context binding " + binding)
			return nil
		}

		secret := `{
			"apiVersion": "v1", "kind": "Secret",
			"metadata": {"name": "d8-cluster-configuration", "namespace": "kube-system"},
			"data": {"cluster-configuration.yaml": "a3ViZXJuZXRlc1ZlcnNpb246IDEuMzMK", "deckhouseDefaultKubernetesVersion": "MS4zMg=="},
			"type": "Opaque"
		}`
		configMap := `{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": {"name": "d8-cluster-kubernetes", "namespace": "kube-system"},
			"data": {"spec": "automaticVersion: \"1.33\"\n", "other": "x"}
		}`
		moduleConfig := `{
			"apiVersion": "deckhouse.io/v1alpha1", "kind": "ModuleConfig",
			"metadata": {"name": "control-plane-manager"},
			"spec": {"enabled": true, "version": 2, "settings": {"kubernetesVersion": "1.33", "enabledFeatureGates": ["A"]}}
		}`

		for _, webhook := range []string{"cpm-moduleconfig-feature-gates", "cpm-k8s-version-feature-gates-mc"} {
			Expect(jq(webhook, "d8-cluster-configuration", secret)).To(Equal(parseJSON(`{
				"data": {"cluster-configuration.yaml": "a3ViZXJuZXRlc1ZlcnNpb246IDEuMzMK", "deckhouseDefaultKubernetesVersion": "MS4zMg=="}
			}`)), webhook)
			// An object without the fields gives nulls, which the handlers read as absent.
			Expect(jq(webhook, "d8-cluster-configuration", `{"kind": "Secret"}`)).To(Equal(parseJSON(`{"data": null}`)), webhook)
		}
		for _, webhook := range []string{"cpm-moduleconfig-feature-gates", "cpm-k8s-version-feature-gates", "cpm-k8s-version-feature-gates-mc"} {
			Expect(jq(webhook, "d8-cluster-kubernetes", configMap)).To(Equal(parseJSON(`{
				"data": {"spec": "automaticVersion: \"1.33\"\n"}
			}`)), webhook)
			Expect(jq(webhook, "d8-cluster-kubernetes", `{"kind": "ConfigMap"}`)).To(Equal(parseJSON(`{"data": {"spec": null}}`)), webhook)
		}
		Expect(jq("cpm-k8s-version-feature-gates", "module-config-control-plane-manager", moduleConfig)).To(Equal(parseJSON(`{
			"spec": {"settings": {"kubernetesVersion": "1.33", "enabledFeatureGates": ["A"]}}
		}`)))
		Expect(jq("cpm-k8s-version-feature-gates", "module-config-control-plane-manager", `{"kind": "ModuleConfig"}`)).
			To(Equal(parseJSON(`{"spec": {"settings": null}}`)))
	})

	It("Should render only the fields the CRD and the operator template accept", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name := range controlPlaneManagerValidationWebhooks {
			object := resource(name)
			undeclared, err := validationwebhook.UndeclaredFields(object)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(undeclared).To(BeEmpty(), name)
			Expect(validationwebhook.LiteralHazards(object)).To(BeEmpty(), name)
		}
	})

})

func parseJSON(text string) interface{} {
	var value interface{}
	Expect(json.Unmarshal([]byte(text), &value)).To(Succeed())
	return value
}
