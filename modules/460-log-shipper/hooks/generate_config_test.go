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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Log shipper :: generate config from crd ::", func() {
	f := HookExecutionConfigInit(`{"logShipper": {"internal": {"activated": false}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLoggingConfig", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ClusterLogDestination", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "PodLoggingConfig", true)

	Context("Simple pair", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_cluster_source_test-source": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test,tier in (cache)",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_test-source_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_10": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_9"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_2"
					],
					"source": " if exists(.parsed_data.\"ap-p\"[0].a) { .app=.parsed_data.\"ap-p\"[0].a } \n .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_4": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_3"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_5": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_6": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_7": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_6"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_8": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_7"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_9": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_8"
					],
					"type": "filter"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_test-source_10"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"pipeline": "testpipe",
					"bulk_action": "index"
				  }
				}
			  }
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_cluster_source_test-source_test-es-dest": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  },
				  "d8_cluster_source_test-source_test-logstash-dest": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  },
				  "d8_cluster_source_test-source_test-loki-dest": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_test-source_test-es-dest_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source_test-es-dest"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-es-dest_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-es-dest_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-es-dest_10": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-es-dest_9"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-es-dest_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_test-es-dest_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_test-es-dest_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-es-dest_2"
					],
					"source": " .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-es-dest_4": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_3"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-es-dest_5": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-es-dest_6": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-es-dest_7": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_6"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-es-dest_8": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_7"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-es-dest_9": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_8"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source_test-logstash-dest"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-logstash-dest_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-logstash-dest_10": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_9"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-logstash-dest_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_test-logstash-dest_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_2"
					],
					"source": " .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-logstash-dest_4": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_3"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_5": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_6": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_7": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_6"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_8": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_7"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-logstash-dest_9": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_8"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source_test-loki-dest"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-loki-dest_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_test-loki-dest_2": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_1"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_3": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_2"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_4": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_3"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_5": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_6": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_test-loki-dest_7": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_6"
					],
					"type": "filter"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_test-source_test-es-dest_10"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"bulk_action": "index"
				  },
				  "d8_cluster_sink_test-logstash-dest": {
					"type": "socket",
					"inputs": [
					  "d8_tf_test-source_test-logstash-dest_10"
					],
					"healthcheck": {
					  "enabled": false
					},
					"address": "192.168.199.252:9009",
					"encoding": {
					  "codec": "json",
					  "timestamp_format": "rfc3339"
					},
					"mode": "tcp",
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false,
					  "verify_certificate": true,
					  "enabled": true
					}
				  },
				  "d8_cluster_sink_test-loki-dest": {
					"type": "loki",
					"inputs": [
					  "d8_tf_test-source_test-loki-dest_7"
					],
					"healthcheck": {
					  "enabled": false
					},
					"encoding": {
					  "codec": "text",
					  "only_fields": [
						"message"
					  ],
					  "timestamp_format": "rfc3339"
					},
					"endpoint": "http://192.168.1.1:9000",
					"labels": {
					  "app": "{{ parsed_data.ap-p[0].a }}",
					  "container": "{{ container }}",
					  "foo": "bar",
					  "image": "{{ image }}",
					  "namespace": "{{ namespace }}",
					  "node": "{{ node }}",
					  "pod": "{{ pod }}",
					  "pod_ip": "{{ pod_ip }}",
					  "pod_labels": "{{ pod_labels }}",
					  "pod_owner": "{{ pod_owner }}",
					  "stream": "{{ stream }}"
					},
					"remove_label_fields": true,
					"out_of_order_action": "rewrite_timestamp"
				  }
				}
			  }
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_clusterns_source_tests-whispers_test-source": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  },
				  "d8_clusterns_source_tests-whistlers_test-source": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whistlers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_test-source_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_clusterns_source_tests-whispers_test-source",
					  "d8_clusterns_source_tests-whistlers_test-source"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_2"
					],
					"source": " .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_4": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_3"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_test-source_4"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"bulk_action": "index"
				  }
				}
			  }
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
			  "sources": {
				"d8_namespaced_source_tests-whispers_whispers-logs_loki-storage": {
				  "type": "kubernetes_logs",
				  "extra_label_selector": "app=test",
				  "extra_field_selector": "metadata.namespace=tests-whispers",
				  "annotation_fields": {
					"container_image": "image",
					"container_name": "container",
					"pod_ip": "pod_ip",
					"pod_labels": "pod_labels",
					"pod_name": "pod",
					"pod_namespace": "namespace",
					"pod_node_name": "node",
					"pod_owner": "pod_owner"
				  },
				  "glob_minimum_cooldown_ms": 1000
				},
				"d8_namespaced_source_tests-whispers_whispers-logs_test-es-dest": {
				  "type": "kubernetes_logs",
				  "extra_label_selector": "app=test",
				  "extra_field_selector": "metadata.namespace=tests-whispers",
				  "annotation_fields": {
					"container_image": "image",
					"container_name": "container",
					"pod_ip": "pod_ip",
					"pod_labels": "pod_labels",
					"pod_name": "pod",
					"pod_namespace": "namespace",
					"pod_node_name": "node",
					"pod_owner": "pod_owner"
				  },
				  "glob_minimum_cooldown_ms": 1000
				}
			  },
			  "transforms": {
				"d8_tf_tests-whispers_whispers-logs_loki-storage_0": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_namespaced_source_tests-whispers_whispers-logs_loki-storage"
				  ],
				  "source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_1": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_0"
				  ],
				  "source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_2": {
				  "condition": "exists(.parsed_data.foo)",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_1"
				  ],
				  "type": "filter"
				},
				  "d8_tf_tests-whispers_whispers-logs_loki-storage_3": {
				  "condition": "!exists(.parsed_data.foo)",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_2"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_4": {
				  "condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_3"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_5": {
				  "condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_4"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_6": {
				  "condition": "match!(.parsed_data.foo, r'^wvrr')",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_5"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_loki-storage_7": {
				  "condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_6"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_0": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_namespaced_source_tests-whispers_whispers-logs_test-es-dest"
				  ],
				  "source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_1": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_0"
				  ],
				  "source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_10": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_9"
				  ],
				  "source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_2": {
				  "hooks": {
					"process": "process"
				  },
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_1"
				  ],
				  "source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
				  "type": "lua",
				  "version": "2"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_3": {
				  "drop_on_abort": false,
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_2"
				  ],
				  "source": " .foo=\"bar\" \n",
				  "type": "remap"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_4": {
				  "condition": "exists(.parsed_data.foo)",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_3"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_5": {
				  "condition": "!exists(.parsed_data.foo)",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_4"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_6": {
				  "condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_5"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_7": {
				  "condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_6"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_8": {
				  "condition": "match!(.parsed_data.foo, r'^wvrr')",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_7"
				  ],
				  "type": "filter"
				},
				"d8_tf_tests-whispers_whispers-logs_test-es-dest_9": {
				  "condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_8"
				  ],
				  "type": "filter"
				}
			  },
			  "sinks": {
				"d8_cluster_sink_loki-storage": {
				  "type": "loki",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_loki-storage_7"
				  ],
				  "healthcheck": {
					"enabled": false
				  },
				  "encoding": {
					"codec": "text",
					"only_fields": [
					  "message"
					],
					"timestamp_format": "rfc3339"
				  },
				  "endpoint": "http://loki.loki:3100",
				  "labels": {
					"container": "{{ container }}",
					"foo": "bar",
					"image": "{{ image }}",
					"namespace": "{{ namespace }}",
					"node": "{{ node }}",
					"pod": "{{ pod }}",
					"pod_ip": "{{ pod_ip }}",
					"pod_labels": "{{ pod_labels }}",
					"pod_owner": "{{ pod_owner }}",
					"stream": "{{ stream }}"
				  },
				  "remove_label_fields": true,
				  "out_of_order_action": "rewrite_timestamp"
				},
				"d8_cluster_sink_test-es-dest": {
				  "type": "elasticsearch",
				  "inputs": [
					"d8_tf_tests-whispers_whispers-logs_test-es-dest_10"
				  ],
				  "healthcheck": {
					"enabled": false
				  },
				  "endpoint": "http://192.168.1.1:9200",
				  "encoding": {
					"timestamp_format": "rfc3339"
				  },
				  "batch": {
					"max_bytes": 10485760,
					"timeout_secs": 1
				  },
				  "compression": "gzip",
				  "index": "logs-%F",
				  "bulk_action": "index"
				}
			  }
			}
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_namespaced_source_tests-whispers_whispers-logs": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_tests-whispers_whispers-logs_0": {
					"group_by": [
					  "file",
					  "stream"
					],
					"inputs": [
					  "d8_namespaced_source_tests-whispers_whispers-logs"
					],
					"merge_strategies": {
					  "message": "concat"
					},
					"starts_when": " matched, err = match(.message, r'^\\{'); if err != null { false; } else { matched; } ",
					"type": "reduce"
				  },
				  "d8_tf_tests-whispers_whispers-logs_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_0"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_tests-whispers_whispers-logs_2": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_1"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_tests-whispers_whispers-logs_3": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_2"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_tests-whispers_whispers-logs_4": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_3"
					],
					"source": " if exists(.parsed_data.\"ap-p\"[0].a) { .app=.parsed_data.\"ap-p\"[0].a } \n .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_tests-whispers_whispers-logs_5": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_4"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_tests-whispers_whispers-logs_5"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"pipeline": "testpipe",
					"bulk_action": "index"
				  }
				}
			  }
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
    indexSettings:
      type: "Datastream"
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_cluster_source_test-source": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test,tier in (cache)",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_test-source_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_10": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_9"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_11": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_10"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_2"
					],
					"source": " if exists(.parsed_data.\"ap-p\"[0].a) { .app=.parsed_data.\"ap-p\"[0].a } \n .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_4": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_3"
					],
					"source": ".\"@timestamp\" = del(.timestamp)",
					"type": "remap"
				  },
				  "d8_tf_test-source_5": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_6": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_7": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_6"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_8": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_7"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_9": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_8"
					],
					"type": "filter"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_test-source_11"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"pipeline": "testpipe",
					"bulk_action": "create"
				  }
				}
			  }
`))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
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
    indexSettings:
      docType: "vector"
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
`, 1))
			f.RunHook()
		})

		It("Should create secret", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("logShipper.internal.activated").Bool()).To(BeTrue())
			secret := f.KubernetesResource("Secret", "d8-log-shipper", "d8-log-shipper-config")
			Expect(secret).To(Not(BeEmpty()))
			config := secret.Field(`data`).Get("vector\\.json").String()
			d, _ := base64.StdEncoding.DecodeString(config)
			Expect(d).Should(MatchJSON(`
			{
				"sources": {
				  "d8_cluster_source_test-source": {
					"type": "kubernetes_logs",
					"extra_label_selector": "app=test,tier in (cache)",
					"extra_field_selector": "metadata.namespace=tests-whispers",
					"annotation_fields": {
					  "container_image": "image",
					  "container_name": "container",
					  "pod_ip": "pod_ip",
					  "pod_labels": "pod_labels",
					  "pod_name": "pod",
					  "pod_namespace": "namespace",
					  "pod_node_name": "node",
					  "pod_owner": "pod_owner"
					},
					"glob_minimum_cooldown_ms": 1000
				  }
				},
				"transforms": {
				  "d8_tf_test-source_0": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_cluster_source_test-source"
					],
					"source": " if exists(.pod_labels.\"controller-revision-hash\") {\n    del(.pod_labels.\"controller-revision-hash\") \n } \n  if exists(.pod_labels.\"pod-template-hash\") { \n   del(.pod_labels.\"pod-template-hash\") \n } \n if exists(.kubernetes) { \n   del(.kubernetes) \n } \n if exists(.file) { \n   del(.file) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_1": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_0"
					],
					"source": " structured, err1 = parse_json(.message) \n if err1 == null { \n   .parsed_data = structured \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_9": {
					"condition": "if exists(.parsed_data.foo) \u0026\u0026 is_string(.parsed_data.foo)\n { \n { matched, err = match(.parsed_data.foo, r'^wvrr')\n if err != null { \n true\n } else {\n !matched\n }}\n } else {\n true\n }",
					"inputs": [
					  "d8_tf_test-source_8"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_10": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_9"
					],
					"source": " if exists(.parsed_data) { \n   del(.parsed_data) \n } \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_2": {
					"hooks": {
					  "process": "process"
					},
					"inputs": [
					  "d8_tf_test-source_1"
					],
					"source": "\nfunction process(event, emit)\n\tif event.log.pod_labels == nil then\n\t\treturn\n\tend\n\tdedot(event.log.pod_labels)\n\temit(event)\nend\nfunction dedot(map)\n\tif map == nil then\n\t\treturn\n\tend\n\tlocal new_map = {}\n\tlocal changed_keys = {}\n\tfor k, v in pairs(map) do\n\t\tlocal dedotted = string.gsub(k, \"%.\", \"_\")\n\t\tif dedotted ~= k then\n\t\t\tnew_map[dedotted] = v\n\t\t\tchanged_keys[k] = true\n\t\tend\n\tend\n\tfor k in pairs(changed_keys) do\n\t\tmap[k] = nil\n\tend\n\tfor k, v in pairs(new_map) do\n\t\tmap[k] = v\n\tend\nend\n",
					"type": "lua",
					"version": "2"
				  },
				  "d8_tf_test-source_3": {
					"drop_on_abort": false,
					"inputs": [
					  "d8_tf_test-source_2"
					],
					"source": " if exists(.parsed_data.\"ap-p\"[0].a) { .app=.parsed_data.\"ap-p\"[0].a } \n .foo=\"bar\" \n",
					"type": "remap"
				  },
				  "d8_tf_test-source_4": {
					"condition": "exists(.parsed_data.foo)",
					"inputs": [
					  "d8_tf_test-source_3"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_5": {
					"condition": "!exists(.parsed_data.fo)",
					"inputs": [
					  "d8_tf_test-source_4"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_6": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { false; } else { includes([\"wvrr\"], data); }; } else { includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_5"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_7": {
					"condition": "if is_boolean(.parsed_data.foo) || is_float(.parsed_data.foo) { data, err = to_string(.parsed_data.foo); if err != null { true; } else { !includes([\"wvrr\"], data); }; } else { !includes([\"wvrr\"], .parsed_data.foo); }",
					"inputs": [
					  "d8_tf_test-source_6"
					],
					"type": "filter"
				  },
				  "d8_tf_test-source_8": {
					"condition": "match!(.parsed_data.foo, r'^wvrr')",
					"inputs": [
					  "d8_tf_test-source_7"
					],
					"type": "filter"
				  }
				},
				"sinks": {
				  "d8_cluster_sink_test-es-dest": {
					"type": "elasticsearch",
					"inputs": [
					  "d8_tf_test-source_10"
					],
					"healthcheck": {
					  "enabled": false
					},
					"endpoint": "http://192.168.1.1:9200",
					"encoding": {
					  "timestamp_format": "rfc3339"
					},
					"batch": {
					  "max_bytes": 10485760,
					  "timeout_secs": 1
					},
					"auth": {
					  "password": "secret",
					  "strategy": "basic",
					  "user": "elastic"
					},
					"tls": {
					  "ca_file": "-----BEGIN CERTIFICATE-----\nMIICwzCCAasCFCjUspjyoopVgNr4tLNRKhRXDfAxMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE0NjA0WhcN\nNDgxMTA3MTE0NjA0WjAeMQswCQYDVQQGEwJSVTEPMA0GA1UEAwwGVGVzdENBMIIB\nIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA3ln6SzVITuVweDTgytxL6NLC\nv+Zyg9wWiVYRVqcghOSAP2XRe2cMbiaNonOhem444dkBEcwxYhXeXAYA47WBHvQG\n+ZFK9oJiBMddiHZf5jTWZC+oJ+6L+HtGdx1K7s3Yh38iC2XtjzU9QBsfeBeJHzYY\neWrmLt6iN6Qt44cywPtJUowjjJiOXPv1z9nT7c/sF/9S1ElXCLWPytwJWSb0eDR+\na1FvgEKWqMarJrEm1iYXKSQYPajXOTShGioHMVC+es1nypszLoweBuV79I/VVv4a\ngVNBa70ibDqs7/w3q2wCb5fZADE832SrWHtcm/InJCkAKys0rI9f89PXyGoYMwID\nAQABMA0GCSqGSIb3DQEBCwUAA4IBAQC4oyj/utVQYkn6yu5Q0MneO+V/NSEHxjNr\nrWNfrnOcSWb8jAQZ3vdZGKQLUokhaSQCJBwarLbAiNUmntogtHDlKgeGqtgU7xVy\nIi1BJW5VLrz8L5GMDGPGcfR3iT7Jh5rzS5QG9aysTx/0jVhStOR5rqjt9hrfk+I/\nT+OMPM5klzsayge9dHLu+yuW0sxxGRO7+9OyV7nOJ4GtLHbqetj0VAB+ijC0zu5M\njLCvoZdJPPUbZeQzqeUnYML+CCDEzBJGIFOWwl53eSnQWlWUiROecawHhnBs1iGb\nSCPD11M34QEfX0pjCNxEIsMKotTzWhEh+/oKrByvumzJjVykrSiy\n-----END CERTIFICATE-----\n",
					  "crt_file": "-----BEGIN CERTIFICATE-----\nMIICtjCCAZ4CFGX3ECr4WwoVPaPZC4fZoN6sbXcOMA0GCSqGSIb3DQEBCwUAMB4x\nCzAJBgNVBAYTAlJVMQ8wDQYDVQQDDAZUZXN0Q0EwHhcNMjEwNjIyMTE1NzE2WhcN\nMzUwMzAxMTE1NzE2WjARMQ8wDQYDVQQDDAZ2ZWN0b3IwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQDGBdHpoX/fC+ZRGEAViOkrxOuoBHk12aSKFWUShIHW\nej04/s1KcdQyELeJY9aC1O5ngXsuZCUCfKSVtq5cr2I5zr4Zisr3BY+reqPUbEeb\nK4PBtEQ9Ibnz6E6LUKwJ+HE1YjibEAnFDejhRQjz0qT5aXGYMwDd+WF1Fvc1ePy/\n8ldG7c3oFg3oFbWZznoVBf39xwYfYtFvpcv5f0mmRVfezjQROgnXcOWFoQxUg0J1\nWQE3LUIGX10sAZsuJp35R7KA/ZHF6Gr8pzfHRcQhvOoeAcJOu6Y0PZ2ppK0azKz/\nqxs+f/aQBfsCtsuvO/Gnb/YaC3TwA2fexe+2AZ6F+SATAgMBAAEwDQYJKoZIhvcN\nAQELBQADggEBAExHd9KAvAYa0vhmZSEdGX7NvHj8AX1OWUAqvbprwbFuBH2fnKX+\nNbFTvWjJCP7dzmtpza1T9Dmo92C4/lZ94W/UsJOF2cHAQPyJvNSvbOTH9a03j8Bh\nimRwfm+LsnotFKxwU4aP+QHG+EPv/AC01wP5a9ei0EYZrHQxuu5l9gTDWcStkkZ9\n/1w4EXgMClYUWgCUGQ6/7/WNBN53cYfyiMPq/UNePeIaRBCmrqnIZP+SZ5p31EQs\nfr2jMkQJ9m7j6XV/DkdXSIl+VgfiXQIrCqSvQuwFWpvpbpTOpRNrXa4ik0BK0mKi\nbbi0LUgo2SpbnHirtiVyP/10Buhf3wHIGGQ=\n-----END CERTIFICATE-----\n",
					  "key_file": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxgXR6aF/3wvmURhAFYjpK8TrqAR5NdmkihVlEoSB1no9OP7N\nSnHUMhC3iWPWgtTuZ4F7LmQlAnyklbauXK9iOc6+GYrK9wWPq3qj1GxHmyuDwbRE\nPSG58+hOi1CsCfhxNWI4mxAJxQ3o4UUI89Kk+WlxmDMA3flhdRb3NXj8v/JXRu3N\n6BYN6BW1mc56FQX9/ccGH2LRb6XL+X9JpkVX3s40EToJ13DlhaEMVINCdVkBNy1C\nBl9dLAGbLiad+UeygP2Rxehq/Kc3x0XEIbzqHgHCTrumND2dqaStGsys/6sbPn/2\nkAX7ArbLrzvxp2/2Ggt08ANn3sXvtgGehfkgEwIDAQABAoIBADUqwt1zmx2L2F7V\nn/8oL1KtIIiQCutGcEMS03xRT3sCfwWahAwE2/BFRMICqEmgWhI4VZZzFOzCAn6f\n+diwzjKvK6M3/J6uQ5DK8MnL+L3UxR9xAxFWyNKQAOau1kInDl5C7OfVOopJ3cj9\n/BVa7Sh6AyHWL9lpZ51EeUNGJLZ0JZufB1QbAWi0NaEZHuaO/QCYNyB8yNMOBGya\nO9LmdyCfO9T/YLZWx/dCN5ZWYrHjTJZDGwOyBwY5B03QafJ+qANNJESMeznyTvDJ\n99whHCIqF4Chp03f7JnPQrBH0HmcC1oAf8LXX9v1/w68JjewU7UHh39Vq6t4cVep\nvXxaWIECgYEA7gCLSSVRPQqoFPApxD05fBjMRgv3kSmipZUM9nW2DvXsTRQCTSSs\nU/bT0nqgAmU7WeR7iAL3eJ1Nnr7yjW8eLZysFYJo32M2lGPgHuVhzRX/vnCNB1CG\ndkYXyd5r+H+vI5elHpo+lUiagv4KbBklBCgD9e4WzdXW7qxI9csMOEMCgYEA1P9R\nxhF5Bh4eGWX7EmC0Tf2UCkOp91uAzPd3f4SPXydKlq02BkpBxVJdCvAW6ZTFgqMu\ntgPqF/+K4M7/HE+b88h7+VvBMU20tqn5c5CbtMGeIM81i/ulE89jRVv/24cxYF+C\niTtVpRxu4IMsNkvp04xB26uphG2NG7CUcfAtI/ECgYEArjXBvonNPDQnsiPVPqpe\nAIMaSw+JaD0kq7U9Zs3ktHC4RfcmdBcq+M7MX92YcAhveC4xae5Z/HSQE2nLm1FB\nsrtijuAFKbayhc3RiGv4uainqVszL652re5CjWX8fEniBdiDabIXqygYyVdwg42o\nNbGgrIxZLtOe3tdHFHtK94cCgYBqWCOq4bRsIoNiqPEnJtM/ETlluozU7IGtVGz8\nZOH0Xzi1bDvJ/i9CZrH/sQmvi9DlPbYnuGKbosHjJlZm+zRhDhsfz/jwNdzhSpI6\nadvj7ruVo/8XKggskOH+kkV3hNNZS7Zv8Aj9y+lr/PIJFfPj5GZJWDbl4JCQX6Ru\nEr1m8QKBgEItNIJKC8KMr2xVPcnj54LYgPobxQrKKNSgEC+E3dDV8LD26vGJfQcI\nL0lPO3VmoYdZBykiAt5CXG5/FK9JCSWCmSY1OFbbgXtx0FjF7sTG8+w+j8mnQ6VP\n7WqSZ053ewFxk/XIXcNwWAQD9nWg3WJMwQADSDgKGctQQW8DOwOV\n-----END RSA PRIVATE KEY-----\n",
					  "verify_hostname": false
					},
					"compression": "gzip",
					"index": "logs-%F",
					"pipeline": "testpipe",
					"bulk_action": "index",
					"doc_type": "vector"
				  }
				}
			  }
`))
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
