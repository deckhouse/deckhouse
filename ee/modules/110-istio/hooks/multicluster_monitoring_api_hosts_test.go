/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: multicluster_monitoring_api_hosts ::", func() {
	f := HookExecutionConfigInit(`{
  "global":{
    "discovery":{
      "clusterUUID":"deadbeef-mycluster",
      "clusterDomain": "my.cluster"
    }
  },
  "istio":{"multicluster":{}, "internal":{"multiclusters": []}}
}`, "")

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(0))
		})
	})

	Context("Empty cluster, minimal settings and multicluster is enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).Should(Equal("expire"))
		})
	})

	Context("Two multiclusters, one hostApi is broken", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
			f.ValuesSetFromYaml("istio.internal.multiclusters", []byte(`
- name: proper-mc
  apiHost: proper-hostname
  apiJWT: some.api.jwt
- name: improper-mc-bad-code
  apiHost: improper-hostname-bad-code
  apiJWT: some.api.jwt
- name: improper-mc-bad-json
  apiHost: improper-hostname-bad-json
  apiJWT: some.api.jwt
- name: improper-mc-wrong-format
  apiHost: improper-hostname-wrong-format
  apiJWT: some.api.jwt
`))
			respMap := map[string]map[string]HTTPMockResponse{
				"proper-hostname": {
					"/api": {
						Response: `{"kind": "APIVersions", "versions": ["v1"]}`,
						Code:     http.StatusOK,
					},
				},
				"improper-hostname-bad-code": {
					"/api": {
						Response: `{"kind": "APIVersions", "versions": ["v1"]}`,
						Code:     http.StatusInternalServerError,
					},
				},
				"improper-hostname-bad-json": {
					"/api": {
						Response: ``,
						Code:     http.StatusOK,
					},
				},
				"improper-hostname-wrong-format": {
					"/api": {
						Response: `{"a":"b"}`,
						Code:     http.StatusOK,
					},
				},
			}
			dependency.TestDC.HTTPClient.DoMock.
				Set(func(req *http.Request) (rp1 *http.Response, err error) {
					host := strings.Split(req.Host, ":")[0]
					uri := req.URL.Path
					mockResponse := respMap[host][uri]
					return &http.Response{
						Header:     map[string][]string{"Content-Type": {"application/json"}},
						StatusCode: mockResponse.Code,
						Body:       io.NopCloser(bytes.NewBufferString(mockResponse.Response)),
					}, nil
				})
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			// there should be 3 log messages
			Expect(strings.Split(strings.Trim(string(f.LogrusOutput.Contents()), "\n"), "\n")).To(HaveLen(3))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot fetch api host improper-hostname-bad-code for IstioMulticluster improper-mc-bad-code (HTTP code 500)"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("cannot unmarshal api host improper-hostname-bad-json response for IstioMulticluster improper-mc-bad-json, error: unexpected end of JSON input"))
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("got wrong response format from api host improper-hostname-wrong-format for IstioMulticluster improper-mc-wrong-format"))

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(5))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  multiclusterMonitoringMetricsGroup,
				Action: "expire",
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMonitoringMetricName,
				Group:  multiclusterMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(0.0),
				Labels: map[string]string{
					"multicluster_name": "proper-mc",
					"api_host":          "proper-hostname",
				},
			}))
			Expect(m[2]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMonitoringMetricName,
				Group:  multiclusterMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "improper-mc-bad-code",
					"api_host":          "improper-hostname-bad-code",
				},
			}))
			Expect(m[3]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMonitoringMetricName,
				Group:  multiclusterMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "improper-mc-bad-json",
					"api_host":          "improper-hostname-bad-json",
				},
			}))
			Expect(m[4]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   multiclusterMonitoringMetricName,
				Group:  multiclusterMonitoringMetricsGroup,
				Action: "set",
				Value:  pointer.Float64Ptr(1.0),
				Labels: map[string]string{
					"multicluster_name": "improper-mc-wrong-format",
					"api_host":          "improper-hostname-wrong-format",
				},
			}))
		})
	})
})
