// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licensing

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promdto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/testing/controller/testclient"
)

// podReader answers the one field selected pod list the reconciler makes. The
// fake client refuses a field selector it has no index for, and the shape of
// that index is not what these tests are about.
type podReader struct {
	client.Reader

	byNode map[string][]corev1.Pod
	err    error
}

func (p *podReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	pods, ok := list.(*corev1.PodList)
	if !ok {
		return p.Reader.List(ctx, list, opts...)
	}
	if p.err != nil {
		return p.err
	}

	var options client.ListOptions
	for _, opt := range opts {
		opt.ApplyToList(&options)
	}
	node := ""
	if options.FieldSelector != nil {
		node, _ = options.FieldSelector.RequiresExactMatch("spec.nodeName")
	}
	pods.Items = p.byNode[node]
	return nil
}

// testEnv is one reconciler with the handles a test needs to look behind it.
type testEnv struct {
	r    *reconciler
	cl   *testclient.Client
	pods *podReader
	reg  *prometheus.Registry
}

func newTestEnv(t *testing.T, vendorKey ed25519.PublicKey, objects ...client.Object) *testEnv {
	t.Helper()

	logger := log.NewNop()
	cl, err := testclient.New(logger, objects)
	if err != nil {
		t.Fatalf("build test client: %v", err)
	}

	// The metrics go into a registry the test owns, so that what was published
	// can be read back the way Prometheus would scrape it.
	reg := prometheus.NewRegistry()
	ms := metricstorage.NewMetricStorage(metricstorage.WithRegistry(reg), metricstorage.WithLogger(logger))
	if err := metrics.RegisterLicensingMetrics(ms); err != nil {
		t.Fatalf("register licensing metrics: %v", err)
	}

	pods := &podReader{Reader: cl}

	return &testEnv{
		r: &reconciler{
			Client:        cl,
			apiReader:     pods,
			metricStorage: ms,
			logger:        logger,
			vendorKeys:    []ed25519.PublicKey{vendorKey},
			thresholds:    licensing.DefaultThresholds(),
			dkpVersion:    "v1.71.0",
			build:         "ee",
			now:           func() time.Time { return testNow },
		},
		cl:   cl,
		pods: pods,
		reg:  reg,
	}
}

// at moves the reconciler clock.
func (e *testEnv) at(now time.Time) { e.r.now = func() time.Time { return now } }

// series returns every sample of one metric family, as scraped.
func (e *testEnv) series(t *testing.T, name string) []*promdto.Metric {
	t.Helper()

	families, err := e.reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			return family.GetMetric()
		}
	}
	return nil
}

// labelOf reads one label off a scraped sample.
func labelOf(m *promdto.Metric, name string) string {
	for _, pair := range m.GetLabel() {
		if pair.GetName() == name {
			return pair.GetValue()
		}
	}
	return ""
}
