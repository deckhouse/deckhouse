/*
Copyright 2021 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: prometheus :: hooks :: get_prometheus_remote_write_crds ", func() {
	const (
		firstPrometheusRemoteWrite = `
---
apiVersion: deckhouse.io/v1
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
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write-second
spec:
  url: https://test-second-victoriametrics.domain.com/api/v1/write
  basicAuth:
    username: user1
    password: pass1
`
		wrongCaPrometheusRemoteWrite = `
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-remote-write-second
spec:
  url: https://test-second-victoriametrics.domain.com/api/v1/write
  tlsConfig:
    ca: "111"
`
		trueCaPrometheusRemoteWrite = `
apiVersion: deckhouse.io/v1
kind: PrometheusRemoteWrite
metadata:
  name: test-ca-true-cert
spec:
  url: https://test-second-victoriametrics.domain.com/api/v1/write
  tlsConfig:
    ca: |
      -----BEGIN CERTIFICATE-----
      MIICCTCCAY6gAwIBAgINAgPluILrIPglJ209ZjAKBggqhkjOPQQDAzBHMQswCQYD
      VQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2VzIExMQzEUMBIG
      A1UEAxMLR1RTIFJvb3QgUjMwHhcNMTYwNjIyMDAwMDAwWhcNMzYwNjIyMDAwMDAw
      WjBHMQswCQYDVQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2Vz
      IExMQzEUMBIGA1UEAxMLR1RTIFJvb3QgUjMwdjAQBgcqhkjOPQIBBgUrgQQAIgNi
      AAQfTzOHMymKoYTey8chWEGJ6ladK0uFxh1MJ7x/JlFyb+Kf1qPKzEUURout736G
      jOyxfi//qXGdGIRFBEFVbivqJn+7kAHjSxm65FSWRQmx1WyRRK2EE46ajA2ADDL2
      4CejQjBAMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW
      BBTB8Sa6oC2uhYHP0/EqEr24Cmf9vDAKBggqhkjOPQQDAwNpADBmAjEA9uEglRR7
      VKOQFhG/hMjqb2sXnh5GmCCbn9MN2azTL818+FsuVbu/3ZL3pAzcMeGiAjEA/Jdm
      ZuVDFhOD3cffL74UOO0BzrEXGhF16b0DjyZ+hOXJYKaV11RZt+cRLInUue4X
      -----END CERTIFICATE-----
`
	)

	f := HookExecutionConfigInit(`{"prometheus":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "PrometheusRemoteWrite", false)

	Context("Synchronization", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(firstPrometheusRemoteWrite, 1))
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
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(firstPrometheusRemoteWrite, 1))
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
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						firstPrometheusRemoteWrite+secondPrometheusRomteWrite, 1,
					))
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
			Context("Apply second PrometheusRemoteWrite CR", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
						trueCaPrometheusRemoteWrite, 1,
					))
					f.RunHook()
				})

				It("Should fill good with true ca prometheusRemoteWrite", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
					Expect(f.ValuesGet("prometheus.internal.remoteWrite").String()).To(MatchJSON(`[
{
 "name":"test-ca-true-cert",
 "spec":{
    "tlsConfig":{
	  "ca":"-----BEGIN CERTIFICATE-----\nMIICCTCCAY6gAwIBAgINAgPluILrIPglJ209ZjAKBggqhkjOPQQDAzBHMQswCQYD\nVQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2VzIExMQzEUMBIG\nA1UEAxMLR1RTIFJvb3QgUjMwHhcNMTYwNjIyMDAwMDAwWhcNMzYwNjIyMDAwMDAw\nWjBHMQswCQYDVQQGEwJVUzEiMCAGA1UEChMZR29vZ2xlIFRydXN0IFNlcnZpY2Vz\nIExMQzEUMBIGA1UEAxMLR1RTIFJvb3QgUjMwdjAQBgcqhkjOPQIBBgUrgQQAIgNi\nAAQfTzOHMymKoYTey8chWEGJ6ladK0uFxh1MJ7x/JlFyb+Kf1qPKzEUURout736G\njOyxfi//qXGdGIRFBEFVbivqJn+7kAHjSxm65FSWRQmx1WyRRK2EE46ajA2ADDL2\n4CejQjBAMA4GA1UdDwEB/wQEAwIBhjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW\nBBTB8Sa6oC2uhYHP0/EqEr24Cmf9vDAKBggqhkjOPQQDAwNpADBmAjEA9uEglRR7\nVKOQFhG/hMjqb2sXnh5GmCCbn9MN2azTL818+FsuVbu/3ZL3pAzcMeGiAjEA/Jdm\nZuVDFhOD3cffL74UOO0BzrEXGhF16b0DjyZ+hOXJYKaV11RZt+cRLInUue4X\n-----END CERTIFICATE-----"
	},
	"url":"https://test-second-victoriametrics.domain.com/api/v1/write"
  }
}]`))
				})
			})
			// Context("Apply second PrometheusRemoteWrite CR", func() {
			// BeforeEach(func() {
			// f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
			// wrongCaPrometheusRemoteWrite, 1,
			// ))
			// f.RunHook()
			// })

			// It("Should get Error for wrong ca prometheusRemoteWrite", func() {
			// Expect(f).To(Not(ExecuteSuccessfully()))
			// })
			// })

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
