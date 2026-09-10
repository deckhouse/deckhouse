/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const podIstiodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: istiod
    istio.io/rev: {{ .Revision }}
  name: istiod-{{ .Revision }}-some-pod-hash
  namespace: d8-istio
spec: {}
status:
  phase: {{ .Phase }}
`

const istioSidecarInjectorGlobalWebhookTemplate = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: d8-istio-sidecar-injector-global
  labels:
    module: istio
    istio.deckhouse.io/full-version: {{ .FullVersion }}
webhooks: []
`

const validationWebHook = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: d8-istio-validator-global
webhooks: []
`

type PodIstiodTemplateParams struct {
	Revision string
	Phase    string
}

func podIstiodYaml(podParams PodIstiodTemplateParams) string {
	return lib.TemplateToYAML(podIstiodTemplate, podParams)
}

func istioSidecarInjectorGlobalWebhookYaml(fullVersion string) string {
	return lib.TemplateToYAML(istioSidecarInjectorGlobalWebhookTemplate, struct{ FullVersion string }{fullVersion})
}

// versionMapFixture builds a versionMap exactly the way versionsDiscovery does, so the
// fixture can not drift from the version cutoffs in discovery_versions.go when they move.
func versionMapFixture(imageAliases ...string) istio_versions.IstioVersionsMap {
	versionMap := istio_versions.IstioVersionsMap{}
	for _, img := range imageAliases {
		ver, err := imageToIstioVersion(img)
		if err != nil {
			panic(err)
		}
		versionMap[ver.version] = ver.info
	}
	return versionMap
}

func mustMarshalJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// isReadyPath returns the values path of the isReady flag of the given Istio version,
// escaping the dot inside the version key.
func isReadyPath(version string) string {
	return versionMapPath + "." + strings.ReplaceAll(version, ".", `\.`) + ".isReady"
}

var _ = Describe("Istio hooks :: discovery istiod health ::", func() {
	// The versions the module actually ships, see modules/110-istio/images/pilot-v1x*.
	// 1.25 supports the operator, 1.27 does not, and 1.29 is the first one able to serve
	// native ambient multicluster.
	versionMap := versionMapFixture("pilotV1x25x2", "pilotV1x27x9", "pilotV1x29x6")

	f := HookExecutionConfigInit(mustMarshalJSON(map[string]interface{}{
		"istio": map[string]interface{}{
			"internal": map[string]interface{}{
				"versionMap": versionMap,
			},
		},
	}), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	// expectReadiness asserts the isReady flag the hook published for every version.
	// The remaining fields belong to discovery_versions.go and are covered by its own test.
	expectReadiness := func(expected map[string]bool) {
		for ver, isReady := range expected {
			Expect(f.ValuesGet(isReadyPath(ver)).Bool()).To(Equal(isReady), "unexpected readiness of version %s", ver)
		}
	}

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Without istiod pods", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
		})
	})

	Context("Without istiod pods but webhook exists", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(validationWebHook))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-istio-validator-global").Exists()).To(BeFalse())
		})
	})

	Context("Istiod pods in `Failed` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x29",
				Phase:    "Failed",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
			// The hook rewrites the whole map, so check once that it touches nothing but isReady.
			Expect(f.ValuesGet(versionMapPath).String()).To(MatchJSON(mustMarshalJSON(versionMap)))
		})
	})

	Context("Istiod pods in `Running` phase and injector webhook with actual full version", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x29",
				Phase:    "Running",
			}) + istioSidecarInjectorGlobalWebhookYaml("1.29.6")))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			expectReadiness(map[string]bool{"1.25": false, "1.27": false, "1.29": true})
		})
	})

	Context("Istiod pods in `Running` phase but injector webhook still has the pre-upgrade full version", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x29",
				Phase:    "Running",
			}) + istioSidecarInjectorGlobalWebhookYaml("1.27.9")))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			// The global revision is not ready until the injector webhook catches up with it,
			// yet isGlobalVersionIstiodReady only looks at the running pod revision.
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			expectReadiness(map[string]bool{"1.25": false, "1.27": false, "1.29": false})
		})
	})

	Context("Both istiod pods with different revisions in `Running` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(
				podIstiodYaml(PodIstiodTemplateParams{
					Revision: "v1x29",
					Phase:    "Running",
				}) +
					podIstiodYaml(PodIstiodTemplateParams{
						Revision: "v1x27",
						Phase:    "Running",
					})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			// A non-global revision needs no injector webhook, the global one does.
			expectReadiness(map[string]bool{"1.25": false, "1.27": true, "1.29": false})
		})
	})

	Context("Istiod pods with `Running` phase and validation webhook exists", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.29")
			f.BindingContexts.Set(f.KubeStateSet(validationWebHook + podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x29",
				Phase:    "Running",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-istio-validator-global").Exists()).To(BeTrue())
		})
	})

	Context("Istiod pods with `Running` phase but with different revision", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.27")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x29",
				Phase:    "Running",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
			expectReadiness(map[string]bool{"1.25": false, "1.27": false, "1.29": true})
		})
	})

})
