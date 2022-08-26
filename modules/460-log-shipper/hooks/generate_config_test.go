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
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func assertConfig(secret object_store.KubeObject, testdataName string) {
	config := secret.Field(`data`).Get("vector\\.json").String()
	d, _ := base64.StdEncoding.DecodeString(config)

	filename := filepath.Join("testdata", testdataName)
	goldenFileData, err := ioutil.ReadFile(filename)
	Expect(err).To(BeNil())

	// Automatically save generated configs to golden files.
	// Use it only if you are aware of changes that caused a diff between generated configs and golden files.
	if os.Getenv("D8_LOG_SHIPPER_SAVE_TESTDATA") == "yes" {
		err := os.WriteFile(filename, d, 0600)
		Expect(err).To(BeNil())
	}

	assert.JSONEq(GinkgoT(), string(goldenFileData), string(d))
}

var _ = Describe("Log shipper :: generate config from crd ::", func() {
	f := HookExecutionConfigInit(`{"logShipper": {"internal": {"activated": false}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLoggingConfig", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLogDestination", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "PodLoggingConfig", true)

	Context("Simple pair", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - tests-whispers
    labelSelector:
      matchLabels:
        app: test
      matchExpressions:
        - key: "tier"
          operator: "In"
          values: ["cache"]
  logFilter:
  - field: foo
    operator: Exists
  - field: fo
    operator: DoesNotExist
  - operator: In
    field: foo
    values:
    - wvrr
  - operator: NotIn
    field: foo
    values:
    - wvrr
  - operator: Regex
    field: foo
    values:
    - ^wvrr
  - operator: NotRegex
    field: foo
    values:
    - ^wvrr
  destinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
    app: "{{ ap-p[0].a }}"
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "simple-pair.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("One source with multiple dests", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - tests-whispers
    labelSelector:
      matchLabels:
        app: test
  logFilter:
  - field: foo
    operator: Exists
  - field: fo
    operator: DoesNotExist
  - operator: In
    field: foo
    values:
    - wvrr
  - operator: NotIn
    field: foo
    values:
    - wvrr
  - operator: Regex
    field: foo
    values:
    - ^wvrr
  - operator: NotRegex
    field: foo
    values:
    - ^wvrr
  labelFilter:
  - field: foo
    operator: Exists
  - field: fo
    operator: DoesNotExist
  - operator: In
    field: test
    values:
    - 111
  - operator: NotIn
    field: test
    values:
    - test-test
  - operator: Regex
    field: test
    values:
    - test.*
  - operator: NotRegex
    field: foo
    values:
    - test.+
  destinationRefs:
    - test-es-dest
    - test-loki-dest
    - test-logstash-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-loki-dest
spec:
  type: Loki
  loki:
    endpoint: http://192.168.1.1:9000
  extraLabels:
    foo: bar
    app: "{{ ap-p[0].a }}"
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-logstash-dest
spec:
  type: Logstash
  logstash:
    endpoint: 192.168.199.252:9009
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
      verifyCertificate: true
  extraLabels:
    foo: bar
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    endpoint: "http://192.168.1.1:9200"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "multiple-dests.json")
		})

		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Multinamespace source with one destination", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - tests-whispers
      - tests-whistlers
    labelSelector:
      matchLabels:
        app: test
  destinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    endpoint: "http://192.168.1.1:9200"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "one-dest.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Namespaced source", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: test
  logFilter:
  - field: foo
    operator: Exists
  - field: foo
    operator: DoesNotExist
  - operator: In
    field: foo
    values:
    - wvrr
  - operator: NotIn
    field: foo
    values:
    - wvrr
  - operator: Regex
    field: foo
    values:
    - ^wvrr
  - operator: NotRegex
    field: foo
    values:
    - ^wvrr
  clusterDestinationRefs:
    - loki-storage
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
  extraLabels:
    foo: bar
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    endpoint: "http://192.168.1.1:9200"
  extraLabels:
    foo: bar
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "namespaced-source.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Namespaced with multiline", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: test
  multilineParser:
    type: MultilineJSON
  clusterDestinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
    app: "{{ ap-p[0].a }}"
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "multiline.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Simple pair with datastream", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - tests-whispers
    labelSelector:
      matchLabels:
        app: test
      matchExpressions:
        - key: "tier"
          operator: "In"
          values: ["cache"]
  logFilter:
  - field: foo
    operator: Exists
  - field: fo
    operator: DoesNotExist
  - operator: In
    field: foo
    values:
    - wvrr
  - operator: NotIn
    field: foo
    values:
    - wvrr
  - operator: Regex
    field: foo
    values:
    - ^wvrr
  - operator: NotRegex
    field: foo
    values:
    - ^wvrr
  destinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    dataStreamEnabled: true
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
    app: "{{ ap-p[0].a }}"
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "pair-datastream.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Simple pair for ES 5.X", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - tests-whispers
    labelSelector:
      matchLabels:
        app: test
      matchExpressions:
        - key: "tier"
          operator: "In"
          values: ["cache"]
  logFilter:
  - field: foo
    operator: Exists
  - field: fo
    operator: DoesNotExist
  - operator: In
    field: foo
    values:
    - wvrr
  - operator: NotIn
    field: foo
    values:
    - wvrr
  - operator: Regex
    field: foo
    values:
    - ^wvrr
  - operator: NotRegex
    field: foo
    values:
    - ^wvrr
  destinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    docType: "_doc"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  extraLabels:
    foo: bar
    app: "{{ ap-p[0].a }}"
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "es-5x.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("Throttle Transform", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: KubernetesPods
  destinationRefs:
    - test-es-dest
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchExpressions:
        - key: environment
          operator: In
          values: ["prod", "test"]
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    tls:
      caFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN3ekNDQWFzQ0ZDalVzcGp5b29wVmdOcjR0TE5SS2hSWERmQXhNQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTBOakEwV2hjTgpORGd4TVRBM01URTBOakEwV2pBZU1Rc3dDUVlEVlFRR0V3SlNWVEVQTUEwR0ExVUVBd3dHVkdWemRFTkJNSUlCCklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEzbG42U3pWSVR1VndlRFRneXR4TDZOTEMKditaeWc5d1dpVllSVnFjZ2hPU0FQMlhSZTJjTWJpYU5vbk9oZW00NDRka0JFY3d4WWhYZVhBWUE0N1dCSHZRRworWkZLOW9KaUJNZGRpSFpmNWpUV1pDK29KKzZMK0h0R2R4MUs3czNZaDM4aUMyWHRqelU5UUJzZmVCZUpIellZCmVXcm1MdDZpTjZRdDQ0Y3l3UHRKVW93ampKaU9YUHYxejluVDdjL3NGLzlTMUVsWENMV1B5dHdKV1NiMGVEUisKYTFGdmdFS1dxTWFySnJFbTFpWVhLU1FZUGFqWE9UU2hHaW9ITVZDK2VzMW55cHN6TG93ZUJ1Vjc5SS9WVnY0YQpnVk5CYTcwaWJEcXM3L3czcTJ3Q2I1ZlpBREU4MzJTcldIdGNtL0luSkNrQUt5czBySTlmODlQWHlHb1lNd0lECkFRQUJNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUM0b3lqL3V0VlFZa242eXU1UTBNbmVPK1YvTlNFSHhqTnIKcldOZnJuT2NTV2I4akFRWjN2ZFpHS1FMVW9raGFTUUNKQndhckxiQWlOVW1udG9ndEhEbEtnZUdxdGdVN3hWeQpJaTFCSlc1VkxyejhMNUdNREdQR2NmUjNpVDdKaDVyelM1UUc5YXlzVHgvMGpWaFN0T1I1cnFqdDlocmZrK0kvClQrT01QTTVrbHpzYXlnZTlkSEx1K3l1VzBzeHhHUk83KzlPeVY3bk9KNEd0TEhicWV0ajBWQUIraWpDMHp1NU0KakxDdm9aZEpQUFViWmVRenFlVW5ZTUwrQ0NERXpCSkdJRk9Xd2w1M2VTblFXbFdVaVJPZWNhd0hobkJzMWlHYgpTQ1BEMTFNMzRRRWZYMHBqQ054RUlzTUtvdFR6V2hFaCsvb0tyQnl2dW16SmpWeWtyU2l5Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      clientCrt:
        crtFile: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN0akNDQVo0Q0ZHWDNFQ3I0V3dvVlBhUFpDNGZab042c2JYY09NQTBHQ1NxR1NJYjNEUUVCQ3dVQU1CNHgKQ3pBSkJnTlZCQVlUQWxKVk1ROHdEUVlEVlFRRERBWlVaWE4wUTBFd0hoY05NakV3TmpJeU1URTFOekUyV2hjTgpNelV3TXpBeE1URTFOekUyV2pBUk1ROHdEUVlEVlFRRERBWjJaV04wYjNJd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFER0JkSHBvWC9mQytaUkdFQVZpT2tyeE91b0JIazEyYVNLRldVU2hJSFcKZWowNC9zMUtjZFF5RUxlSlk5YUMxTzVuZ1hzdVpDVUNmS1NWdHE1Y3IySTV6cjRaaXNyM0JZK3JlcVBVYkVlYgpLNFBCdEVROUlibno2RTZMVUt3SitIRTFZamliRUFuRkRlamhSUWp6MHFUNWFYR1lNd0RkK1dGMUZ2YzFlUHkvCjhsZEc3YzNvRmczb0ZiV1p6bm9WQmYzOXh3WWZZdEZ2cGN2NWYwbW1SVmZlempRUk9nblhjT1dGb1F4VWcwSjEKV1FFM0xVSUdYMTBzQVpzdUpwMzVSN0tBL1pIRjZHcjhwemZIUmNRaHZPb2VBY0pPdTZZMFBaMnBwSzBhekt6LwpxeHMrZi9hUUJmc0N0c3V2Ty9HbmIvWWFDM1R3QTJmZXhlKzJBWjZGK1NBVEFnTUJBQUV3RFFZSktvWklodmNOCkFRRUxCUUFEZ2dFQkFFeEhkOUtBdkFZYTB2aG1aU0VkR1g3TnZIajhBWDFPV1VBcXZicHJ3YkZ1QkgyZm5LWCsKTmJGVHZXakpDUDdkem10cHphMVQ5RG1vOTJDNC9sWjk0Vy9Vc0pPRjJjSEFRUHlKdk5TdmJPVEg5YTAzajhCaAppbVJ3Zm0rTHNub3RGS3h3VTRhUCtRSEcrRVB2L0FDMDF3UDVhOWVpMEVZWnJIUXh1dTVsOWdURFdjU3Rra1o5Ci8xdzRFWGdNQ2xZVVdnQ1VHUTYvNy9XTkJONTNjWWZ5aU1QcS9VTmVQZUlhUkJDbXJxbklaUCtTWjVwMzFFUXMKZnIyak1rUUo5bTdqNlhWL0RrZFhTSWwrVmdmaVhRSXJDcVN2UXV3RldwdnBicFRPcFJOclhhNGlrMEJLMG1LaQpiYmkwTFVnbzJTcGJuSGlydGlWeVAvMTBCdWhmM3dISUdHUT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
        keyFile: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBeGdYUjZhRi8zd3ZtVVJoQUZZanBLOFRycUFSNU5kbWtpaFZsRW9TQjFubzlPUDdOClNuSFVNaEMzaVdQV2d0VHVaNEY3TG1RbEFueWtsYmF1WEs5aU9jNitHWXJLOXdXUHEzcWoxR3hIbXl1RHdiUkUKUFNHNTgraE9pMUNzQ2ZoeE5XSTRteEFKeFEzbzRVVUk4OUtrK1dseG1ETUEzZmxoZFJiM05Yajh2L0pYUnUzTgo2QllONkJXMW1jNTZGUVg5L2NjR0gyTFJiNlhMK1g5SnBrVlgzczQwRVRvSjEzRGxoYUVNVklOQ2RWa0JOeTFDCkJsOWRMQUdiTGlhZCtVZXlnUDJSeGVocS9LYzN4MFhFSWJ6cUhnSENUcnVtTkQyZHFhU3RHc3lzLzZzYlBuLzIKa0FYN0FyYkxyenZ4cDIvMkdndDA4QU5uM3NYdnRnR2VoZmtnRXdJREFRQUJBb0lCQURVcXd0MXpteDJMMkY3VgpuLzhvTDFLdElJaVFDdXRHY0VNUzAzeFJUM3NDZndXYWhBd0UyL0JGUk1JQ3FFbWdXaEk0VlpaekZPekNBbjZmCitkaXd6akt2SzZNMy9KNnVRNURLOE1uTCtMM1V4Ujl4QXhGV3lOS1FBT2F1MWtJbkRsNUM3T2ZWT29wSjNjajkKL0JWYTdTaDZBeUhXTDlscFo1MUVlVU5HSkxaMEpadWZCMVFiQVdpME5hRVpIdWFPL1FDWU55Qjh5Tk1PQkd5YQpPOUxtZHlDZk85VC9ZTFpXeC9kQ041WldZckhqVEpaREd3T3lCd1k1QjAzUWFmSitxQU5OSkVTTWV6bnlUdkRKCjk5d2hIQ0lxRjRDaHAwM2Y3Sm5QUXJCSDBIbWNDMW9BZjhMWFg5djEvdzY4Smpld1U3VUhoMzlWcTZ0NGNWZXAKdlh4YVdJRUNnWUVBN2dDTFNTVlJQUXFvRlBBcHhEMDVmQmpNUmd2M2tTbWlwWlVNOW5XMkR2WHNUUlFDVFNTcwpVL2JUMG5xZ0FtVTdXZVI3aUFMM2VKMU5ucjd5alc4ZUxaeXNGWUpvMzJNMmxHUGdIdVZoelJYL3ZuQ05CMUNHCmRrWVh5ZDVyK0grdkk1ZWxIcG8rbFVpYWd2NEtiQmtsQkNnRDllNFd6ZFhXN3F4STljc01PRU1DZ1lFQTFQOVIKeGhGNUJoNGVHV1g3RW1DMFRmMlVDa09wOTF1QXpQZDNmNFNQWHlkS2xxMDJCa3BCeFZKZEN2QVc2WlRGZ3FNdQp0Z1BxRi8rSzRNNy9IRStiODhoNytWdkJNVTIwdHFuNWM1Q2J0TUdlSU04MWkvdWxFODlqUlZ2LzI0Y3hZRitDCmlUdFZwUnh1NElNc05rdnAwNHhCMjZ1cGhHMk5HN0NVY2ZBdEkvRUNnWUVBcmpYQnZvbk5QRFFuc2lQVlBxcGUKQUlNYVN3K0phRDBrcTdVOVpzM2t0SEM0UmZjbWRCY3ErTTdNWDkyWWNBaHZlQzR4YWU1Wi9IU1FFMm5MbTFGQgpzcnRpanVBRktiYXloYzNSaUd2NHVhaW5xVnN6TDY1MnJlNUNqV1g4ZkVuaUJkaURhYklYcXlnWXlWZHdnNDJvCk5iR2dySXhaTHRPZTN0ZEhGSHRLOTRjQ2dZQnFXQ09xNGJSc0lvTmlxUEVuSnRNL0VUbGx1b3pVN0lHdFZHejgKWk9IMFh6aTFiRHZKL2k5Q1pySC9zUW12aTlEbFBiWW51R0tib3NIakpsWm0relJoRGhzZnovandOZHpoU3BJNgphZHZqN3J1Vm8vOFhLZ2dza09IK2trVjNoTk5aUzdadjhBajl5K2xyL1BJSkZmUGo1R1pKV0RibDRKQ1FYNlJ1CkVyMW04UUtCZ0VJdE5JSktDOEtNcjJ4VlBjbmo1NExZZ1BvYnhRcktLTlNnRUMrRTNkRFY4TEQyNnZHSmZRY0kKTDBsUE8zVm1vWWRaQnlraUF0NUNYRzUvRks5SkNTV0NtU1kxT0ZiYmdYdHgwRmpGN3NURzgrdytqOG1uUTZWUAo3V3FTWjA1M2V3RnhrL1hJWGNOd1dBUUQ5bldnM1dKTXdRQURTRGdLR2N0UVFXOERPd09WCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==
      verifyHostname: false
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0
  rateLimit:
    linesPerMinute: 500
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "throttle.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("File to Elasticsearch", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: File
  file:
    include: ["/var/log/kube-audit/audit.log"]
  destinationRefs:
    - test-es-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-es-dest
spec:
  type: Elasticsearch
  elasticsearch:
    index: "logs-%F"
    pipeline: "testpipe"
    endpoint: "http://192.168.1.1:9200"
    tls:
      verifyCertificate: false
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "file-to-elastic.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})

	Context("File to Vector", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: test-source
spec:
  type: File
  file:
    include: ["/var/log/kube-audit/audit.log"]
  destinationRefs:
    - test-vector-dest
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-vector-dest
spec:
  type: Vector
  vector:
    endpoint: "192.168.1.1:9200"
    tls:
      verifyCertificate: false
      verifyHostname: false
---
`))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))

			assertConfig(secret, "file-to-vector.json")
		})
		Context("With deleting object", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})
			It("Should delete secret and deactivate module", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config").Exists()).To(BeFalse())
			})
		})
	})
})
