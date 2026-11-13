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
	"encoding/base64"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// webhook-handler/ runs the shell-operator that serves the ValidationWebhook and ConversionWebhook
// objects of the cluster. The handler registers its webhooks against Services by name and serves them
// with certificates from a mounted Secret, so most of these tests check that the objects of the
// directory fit together. The PodMonitor is checked in module_test.go.

const webhookHandler = "webhook-handler"

var _ = Describe("Module :: deckhouse :: webhook-handler ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	render := func() {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	deployment := func() object_store.KubeObject {
		deployment := f.KubernetesResource("Deployment", "d8-system", webhookHandler)
		Expect(deployment.Exists()).To(BeTrue())
		return deployment
	}

	handler := func() library.KubeResult {
		return deployment().Field(`spec.template.spec.containers.#(name=="handler")`)
	}

	env := func(name string) library.KubeResult {
		return deployment().Field(`spec.template.spec.containers.#(name=="handler").env.#(name=="` + name + `")`)
	}

	Context("in a bootstrapped cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterIsBootstrapped", true)
			render()
		})

		It("serves the webhooks behind the Services it registers them with", func() {
			Expect(deployment().Field("spec.template.metadata.labels.app").String()).To(Equal(webhookHandler))
			for variable, service := range map[string]string{
				"VALIDATING_WEBHOOK_SERVICE_NAME": "validating-webhook-handler",
				"CONVERSION_WEBHOOK_SERVICE_NAME": "conversion-webhook-handler",
			} {
				Expect(env(variable).Get("value").String()).To(Equal(service))
				svc := f.KubernetesResource("Service", "d8-system", service)
				Expect(svc.Exists()).To(BeTrue(), "the %s Service should be rendered", service)
				Expect(svc.Field("spec.selector").String()).To(MatchJSON(`{"app":"webhook-handler"}`))

				Expect(svc.Field("spec.ports").Array()).To(HaveLen(1))
				Expect(svc.Field("spec.ports.0.port").Int()).To(Equal(int64(443)))
				targetPort := svc.Field("spec.ports.0.targetPort").String()
				Expect(handler().Get(`ports.#(name=="`+targetPort+`")`).Exists()).To(BeTrue(),
					"the %s Service should send to a port of the handler, not to %s", service, targetPort)
			}
		})

		It("serves the webhooks with the certificate the module issued", func() {
			secret := f.KubernetesResource("Secret", "d8-system", "webhook-handler-certs")
			Expect(secret.Exists()).To(BeTrue())
			Expect(secret.Field("type").String()).To(Equal("kubernetes.io/tls"))
			for key, value := range map[string]string{
				`tls\.crt`: "deckhouse.internal.webhookHandlerCert.crt",
				`tls\.key`: "deckhouse.internal.webhookHandlerCert.key",
				`ca\.crt`:  "deckhouse.internal.webhookHandlerCert.ca",
			} {
				decoded, err := base64.StdEncoding.DecodeString(secret.Field("data." + key).String())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(decoded)).To(Equal(f.ValuesGet(value).String()), key)
			}

			Expect(deployment().Field(`spec.template.spec.volumes.#(name=="certs").secret.secretName`).String()).To(Equal("webhook-handler-certs"))
			Expect(handler().Get(`volumeMounts.#(name=="certs").mountPath`).String()).To(Equal("/certs/"))
			for _, kind := range []string{"VALIDATING", "CONVERSION"} {
				Expect(env(kind + "_WEBHOOK_SERVER_CERT").Get("value").String()).To(Equal("/certs/tls.crt"))
				Expect(env(kind + "_WEBHOOK_SERVER_KEY").Get("value").String()).To(Equal("/certs/tls.key"))
				Expect(env(kind + "_WEBHOOK_CA").Get("value").String()).To(Equal("/certs/ca.crt"))
			}
		})

		It("rolls the handler out again when the certificate changes", func() {
			checksum := func() string {
				return deployment().Field(`spec.template.metadata.annotations.checksum/config`).String()
			}

			issued := checksum()
			Expect(issued).ToNot(BeEmpty())

			f.ValuesSet("deckhouse.internal.webhookHandlerCert.crt", "renewed")
			render()
			Expect(checksum()).ToNot(Equal(issued))
		})

		It("fails closed once the cluster is bootstrapped", func() {
			Expect(env("VALIDATING_WEBHOOK_FAILURE_POLICY").Exists()).To(BeFalse(),
				"without the variable shell-operator defaults the webhooks to failurePolicy Fail")
		})

		It("tells the hooks which modules are enabled", func() {
			Expect(env("ENABLED_MODULES").Get("value").String()).
				To(Equal(strings.Join(f.ValuesGet("global.enabledModules").AsStringSlice(), " ")))
		})

		It("names no instance classes in a static cluster", func() {
			Expect(env("INSTANCE_CLASS_NAME").Exists()).To(BeFalse())
		})
	})

	It("fails open until the cluster is bootstrapped, so that a handler that is not up yet does not stop the bootstrap", func() {
		render()

		Expect(env("VALIDATING_WEBHOOK_FAILURE_POLICY").Get("value").String()).To(Equal("Ignore"))
	})

	It("names the instance classes of the cloud provider in a cloud cluster", func() {
		f.ValuesSet("global.clusterConfiguration.clusterType", "Cloud")
		f.ValuesSetFromYaml("global.clusterConfiguration.cloud", `{provider: OpenStack}`)
		render()

		Expect(env("INSTANCE_CLASS_NAME").Get("value").String()).To(Equal("openstackinstanceclasses"))
	})

	Context("resources", func() {
		requests := func() library.KubeResult {
			return deployment().Field(`spec.template.spec.containers.#(name=="handler").resources.requests`)
		}

		It("leaves the CPU and memory of the handler to the VPA when the VPA is enabled", func() {
			render()

			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", webhookHandler)
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.targetRef").String()).To(MatchJSON(`{"apiVersion":"apps/v1","kind":"Deployment","name":"webhook-handler"}`))
			Expect(vpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))
			policy := vpa.Field(`spec.resourcePolicy.containerPolicies.#(containerName=="handler")`)
			Expect(policy.Get("minAllowed").String()).To(MatchJSON(`{"cpu":"50m","memory":"100Mi"}`))
			Expect(policy.Get("maxAllowed").String()).To(MatchJSON(`{"cpu":"200m","memory":"100Mi"}`))

			Expect(requests().Get("cpu").Exists()).To(BeFalse(), "the VPA sets the requests")
			Expect(requests().Get("memory").Exists()).To(BeFalse(), "the VPA sets the requests")
			Expect(requests().Get("ephemeral-storage").Exists()).To(BeTrue())
		})

		It("requests the minimum of the VPA itself when the VPA is disabled", func() {
			f.ValuesSetFromYaml("global.enabledModules", `["prometheus", "operator-prometheus"]`)
			render()

			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-system", webhookHandler).Exists()).To(BeFalse())
			Expect(requests().Get("cpu").String()).To(Equal("50m"))
			Expect(requests().Get("memory").String()).To(Equal("100Mi"))
			Expect(requests().Get("ephemeral-storage").Exists()).To(BeTrue())
		})
	})

	Context("high availability", func() {
		pdb := func() object_store.KubeObject {
			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-system", webhookHandler)
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.selector.matchLabels").String()).To(MatchJSON(`{"app":"webhook-handler"}`))
			return pdb
		}

		It("runs two replicas and keeps one of them through a disruption", func() {
			f.ValuesSet("deckhouse.highAvailability", true)
			render()

			Expect(deployment().Field("spec.replicas").Int()).To(Equal(int64(2)))
			Expect(pdb().Field("spec.minAvailable").Int()).To(Equal(int64(1)))
		})

		It("runs one replica and lets a disruption take it", func() {
			f.ValuesSet("deckhouse.highAvailability", false)
			render()

			Expect(deployment().Field("spec.replicas").Int()).To(Equal(int64(1)))
			Expect(pdb().Field("spec.minAvailable").Int()).To(Equal(int64(0)),
				"a budget that keeps the only replica would block the drain of its node")
		})
	})

	Context("access", func() {
		BeforeEach(func() {
			render()
		})

		grants := func() map[string][]string {
			return roleGrants(f.KubernetesGlobalResource("ClusterRole", "d8:deckhouse:webhook-handler"))
		}

		It("runs as its own service account, bound to its own role", func() {
			Expect(deployment().Field("spec.template.spec.serviceAccountName").String()).To(Equal(webhookHandler))
			Expect(f.KubernetesResource("ServiceAccount", "d8-system", webhookHandler).Exists()).To(BeTrue())

			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:deckhouse:webhook-handler")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"d8:deckhouse:webhook-handler"}`))
			Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"webhook-handler","namespace":"d8-system"}]`))
		})

		It("may register its webhooks and read what they look at, and change nothing else", func() {
			Expect(grants()).To(Equal(map[string][]string{
				// Registering the webhooks.
				"admissionregistration.k8s.io/validatingwebhookconfigurations": {"create", "list", "update"},
				"apiextensions.k8s.io/customresourcedefinitions":               {"get", "list", "patch", "update"},
				"deckhouse.io/validationwebhooks":                              {"get", "list", "update"},
				"deckhouse.io/conversionwebhooks":                              {"get", "list", "update"},

				// What the hooks watch.
				"deckhouse.io/*": rbacReadVerbs,
				"secrets":        rbacReadVerbs,
				"endpoints":      rbacReadVerbs,
				"configmaps":     rbacReadVerbs,
				"namespaces":     rbacReadVerbs,
				"nodes":          rbacReadVerbs,
				"services":       rbacReadVerbs,

				"rbac.authorization.k8s.io/clusterroles":            rbacReadVerbs,
				"rbac.authorization.k8s.io/clusterrolebindings":     rbacReadVerbs,
				"network.deckhouse.io/metalloadbalancerclasses":     rbacReadVerbs,
				"control-plane.deckhouse.io/controlplaneoperations": rbacReadVerbs,

				// What the hooks read on demand.
				"pods":                   {"get"},
				"apps/deployments":       {"get"},
				"apps/statefulsets":      {"get"},
				"apps/replicasets":       {"get"},
				"replicationcontrollers": {"get"},
			}), "the pods and the parents of a scale are read one by one, never watched")
		})

		// The heritage-subresources webhook reads the object behind each request, so a kind routed to
		// it that the handler may not read would have every request denied with a read error.
		It("may read the object behind every request the heritage webhook routes to it", func() {
			webhook := f.KubernetesGlobalResource("ValidationWebhook", "heritage-subresources")
			Expect(webhook.Exists()).To(BeTrue())

			rules := webhook.Field("validationObject.rules").Array()
			Expect(rules).ToNot(BeEmpty())
			for _, rule := range rules {
				for _, group := range rule.Get("apiGroups").Array() {
					for _, resource := range rule.Get("resources").Array() {
						parent, _, _ := strings.Cut(resource.String(), "/")
						key := parent
						if group.String() != "" {
							key = group.String() + "/" + parent
						}
						Expect(grants()).To(HaveKeyWithValue(key, ContainElement("get")),
							"the webhook reviews %s and reads %s", resource.String(), key)
					}
				}
			}
		})
	})
})
