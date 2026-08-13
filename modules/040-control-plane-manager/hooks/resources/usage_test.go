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

package resources

// The metrics API layer, which the hook-level tests replace with a stub: unit
// conversion and the sanity filters are where an off-by-1000 hides, and nothing
// above this file would notice one.

import (
	"context"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restfake "k8s.io/client-go/rest/fake"
)

func metricsAPIReturning(status int, body string) rest.Interface {
	return &restfake.RESTClient{
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		GroupVersion:         schema.GroupVersion{Group: "custom.metrics.k8s.io", Version: "v1beta1"},
		VersionedAPIPath:     "/apis/custom.metrics.k8s.io/v1beta1",
		Client: restfake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

var _ = Describe("Module hooks :: control-plane-manager :: resources_requests_autotune :: metrics API", func() {
	DescribeTable("clampUsage",
		func(raw float64, kind resourceKind, want int64) {
			Expect(clampUsage(raw, kind)).To(Equal(want))
		},
		Entry("cores to millicpu", 0.25, resourceCPU, int64(250)),
		Entry("millicpu rounds up, never down", 0.5001, resourceCPU, int64(501)),
		Entry("cpu below the floor", 0.0001, resourceCPU, autotuneMinMilliCPU),
		Entry("bytes stay bytes", float64(500000000), resourceMemory, int64(500000000)),
		Entry("bytes round to the nearest", 20000000.6, resourceMemory, int64(20000001)),
		Entry("memory below the floor", float64(1), resourceMemory, autotuneMinMemoryBytes),
	)

	DescribeTable("fetchPodMetric",
		func(status int, body string, wantValue float64, wantOK, wantErr bool) {
			value, ok, err := fetchPodMetric(context.Background(),
				metricsAPIReturning(status, body), "kube-apiserver-master-0", "d8-cpm-autotune-cpu")
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(ok).To(Equal(wantOK))
			Expect(value).To(Equal(wantValue))
		},
		Entry("a value", http.StatusOK, `{"items":[{"value":"250m"}]}`, 0.25, true, false),
		Entry("bytes as a plain number", http.StatusOK, `{"items":[{"value":"1500000000"}]}`, float64(1500000000), true, false),
		// A miss, not a failure: the adapter answered, it just has nothing yet.
		Entry("empty list", http.StatusOK, `{"items":[]}`, float64(0), false, false),
		Entry("no series for the pod", http.StatusNotFound, `{}`, float64(0), false, false),
		// Nonsense that must never reach clampUsage.
		Entry("zero", http.StatusOK, `{"items":[{"value":"0"}]}`, float64(0), false, true),
		Entry("negative", http.StatusOK, `{"items":[{"value":"-5"}]}`, float64(0), false, true),
		Entry("malformed body", http.StatusOK, `not json`, float64(0), false, true),
		Entry("adapter is broken", http.StatusInternalServerError, `{}`, float64(0), false, true),
	)
})
