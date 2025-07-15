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
  headers:
    X-Scope-OrgID: "org1"
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
  headers:
    X-Scope-OrgID: "org1"
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
      "headers": {
          "X-Scope-OrgID": "org1"
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
      "headers": {
          "X-Scope-OrgID": "org1"
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
      "headers": {
          "X-Scope-OrgID": "org1"
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
      "headers": {
          "X-Scope-OrgID": "org1"
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
