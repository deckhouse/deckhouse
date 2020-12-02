package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: get_prometheus_remote_write_crds ", func() {
	const (
		firstPrometheusRemoteWrite = `
---
apiVersion: deckhouse.io/v1alpha1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write
spec:
  url: https://test-victoriametrics.domain.com/api/v1/write
  basicAuth:
    username: username
    password: password
  writeRelabelConfigs:
    - sourceLabels: [__name__]
      regex: prometheus_build_.*
      action: keep
`
		secondPrometheusRomteWrite = `
---
apiVersion: deckhouse.io/v1alpha1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write-second
spec:
  url: https://test-second-victoriametrics.domain.com/api/v1/write
  basicAuth:
    username: user1
    password: pass1
`
	)

	f := HookExecutionConfigInit(`{"prometheus":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "PrometheusRemoteWrite", false)

	Context("Synchronization", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(firstPrometheusRemoteWrite))
			f.RunHook()
		})

		It("Should fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
			Expect(f.ValuesGet("prometheus.internal.remoteWrite").String()).To(MatchJSON(`
[
  {
    "name": "test-remote-write",
    "spec": {
      "basicAuth": {
        "password": "password",
        "username": "username"
      },
      "url": "https://test-victoriametrics.domain.com/api/v1/write",
      "writeRelabelConfigs": [
        {
          "action": "keep",
          "regex": "prometheus_build_.*",
          "sourceLabels": [
            "__name__"
          ]
        }
      ]
    }
  }
]
`))
		})
	})
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		Context("PrometheusRemoteWrite CR created", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(firstPrometheusRemoteWrite))
				f.RunHook()
			})
			It("Should fill internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
				Expect(f.ValuesGet("prometheus.internal.remoteWrite").String()).To(MatchJSON(`
[
  {
    "name": "test-remote-write",
    "spec": {
      "basicAuth": {
        "password": "password",
        "username": "username"
      },
      "url": "https://test-victoriametrics.domain.com/api/v1/write",
      "writeRelabelConfigs": [
        {
          "action": "keep",
          "regex": "prometheus_build_.*",
          "sourceLabels": [
            "__name__"
          ]
        }
      ]
    }
  }
]
`))
			})

			Context("Apply second PrometheusRemoteWrite CR", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(firstPrometheusRemoteWrite + secondPrometheusRomteWrite))
					f.RunHook()
				})

				It("Should fill internal values with two prometheusRemoteWrite", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
					Expect(f.ValuesGet("prometheus.internal.remoteWrite").String()).To(MatchJSON(`
[
  {
	"name": "test-remote-write",
	"spec": {
	  "basicAuth": {
		"password": "password",
		"username": "username"
	  },
	  "url": "https://test-victoriametrics.domain.com/api/v1/write",
	  "writeRelabelConfigs": [
		{
		  "action": "keep",
		  "regex": "prometheus_build_.*",
		  "sourceLabels": [
			"__name__"
		  ]
		}
	  ]
	}
  },
  {
	"name": "test-remote-write-second",
	"spec": {
	  "basicAuth": {
		"password": "pass1",
		"username": "user1"
	  },
	  "url": "https://test-second-victoriametrics.domain.com/api/v1/write"
	}
  }
]
`))
				})
			})

			Context("With deleting PrometheusRemoteWrite CR", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(""))
					f.RunHook()
				})
				It("Should delete entry from internal values", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
					Expect(f.ValuesGet("prometheus.internal.remoteWrite").String()).To(Equal("[]"))
				})
			})
		})
	})
})
