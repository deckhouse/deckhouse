/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: istio :: hooks :: patch_injector_configmap ", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)
	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Injector configmap for revision v1x33, limits are exists", func() {
		cm := `
---
apiVersion: v1
data:
  config: test_config_data
  values: |-
    {
      "global": {
        "proxy": {
          "resources": {
            "limits": {
              "cpu": "2000m",
              "memory": "1024Mi"
            },
            "requests": {
              "cpu": "100m",
              "memory": "128Mi"
            }
          }
        }
      }
    }
kind: ConfigMap
metadata:
  labels:
    install.operator.istio.io/owning-resource: v1x33
    install.operator.istio.io/owning-resource-namespace: d8-istio
    istio.io/rev: v1x13
    operator.istio.io/component: Pilot
    operator.istio.io/managed: Reconcile
    operator.istio.io/version: 1.33.7
    release: istio
  name: istio-sidecar-injector-v1x33-limits-exists
  namespace: d8-istio
`

		patchedValues := `
{
  "global": {
    "proxy": {
      "resources": {
        "limits": {
          "memory": "1024Mi"
        },
        "requests": {
          "cpu": "100m",
          "memory": "128Mi"
        }
      }
    }
  }
}
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-limits-exists").ToYaml()).NotTo(MatchYAML(cm))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-limits-exists").Field("data.config").String()).To(Equal("test_config_data"))
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-limits-exists").Field("data.values").String()).To(MatchJSON(patchedValues))
		})
	})

	Context("Injector configmap for revision v1x33, limits are absent", func() {
		cm := `
---
apiVersion: v1
data:
  config: test_config_data
  values: |-
    {
      "global": {
        "proxy": {
          "resources": {
            "requests": {
              "cpu": "100m",
              "memory": "128Mi"
            }
          }
        }
      }
    }
kind: ConfigMap
metadata:
  labels:
    install.operator.istio.io/owning-resource: v1x33
    install.operator.istio.io/owning-resource-namespace: d8-istio
    istio.io/rev: v1x13
    operator.istio.io/component: Pilot
    operator.istio.io/managed: Reconcile
    operator.istio.io/version: 1.33.7
    release: istio
  name: istio-sidecar-injector-v1x33-limits-absent
  namespace: d8-istio
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-limits-absent").ToYaml()).To(MatchYAML(cm))
		})
	})

	Context("Configmap with values field but with wrong data", func() {
		cm := `
---
apiVersion: v1
data:
  config: test_config_data
  values: test_values
kind: ConfigMap
metadata:
  labels:
    install.operator.istio.io/owning-resource: v1x33
    install.operator.istio.io/owning-resource-namespace: d8-istio
    istio.io/rev: v1x13
    operator.istio.io/component: Pilot
    operator.istio.io/managed: Reconcile
    operator.istio.io/version: 1.33.7
    release: istio
  name: istio-sidecar-injector-v1x33-wrong-data
  namespace: d8-istio
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-wrong-data").ToYaml()).To(MatchYAML(cm))
		})
	})

	Context("Configmap without needed fields", func() {
		cm := `
---
apiVersion: v1
data:
  config1: test_config_data
  values1: test_values
kind: ConfigMap
metadata:
  labels:
    install.operator.istio.io/owning-resource: v1x33
    install.operator.istio.io/owning-resource-namespace: d8-istio
    istio.io/rev: v1x13
    operator.istio.io/component: Pilot
    operator.istio.io/managed: Reconcile
    operator.istio.io/version: 1.33.7
    release: istio
  name: istio-sidecar-injector-v1x33-no-fields
  namespace: d8-istio
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-istio", "istio-sidecar-injector-v1x33-no-fields").ToYaml()).To(MatchYAML(cm))
		})
	})

})
