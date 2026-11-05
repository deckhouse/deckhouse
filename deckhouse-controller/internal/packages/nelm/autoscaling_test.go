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

package nelm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// webManifest renders the Deployment web with the container app.
const webManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: app
`

// webAutoscalers render a HorizontalPodAutoscaler and a VerticalPodAutoscaler of web.
const webAutoscalers = `
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web
spec:
  scaleTargetRef:
    kind: Deployment
    name: web
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: web
spec:
  targetRef:
    kind: Deployment
    name: web
`

// rangedWeb asks for a replicas range of web and a cpu range of its container app.
var rangedWeb = []resourcesize.Request{{
	Kind:        "Deployment",
	Name:        "web",
	MinReplicas: new(int32(2)),
	MaxReplicas: new(int32(5)),
	Containers: []resourcesize.Container{{
		Name: "app",
		Min:  map[string]string{"cpu": "100m"},
		Max:  map[string]string{"cpu": "1"},
	}},
}}

// fixedWeb asks for fixed replicas of web and a fixed cpu request of its container app.
var fixedWeb = []resourcesize.Request{{
	Kind:       "Deployment",
	Name:       "web",
	Replicas:   new(int32(3)),
	Containers: []resourcesize.Container{{Name: "app", Requests: map[string]string{"cpu": "100m"}}},
}}

// webTargets are the replicas of web and the cpu of its container app.
var webTargets = []resourcesize.Target{
	{Kind: "Deployment", Name: "web", Resource: "replicas"},
	{Kind: "Deployment", Name: "web", Container: "app", Resource: "cpu"},
}

// Ranges without autoscalers fall back to fixed values.
func TestMeetAutoscalersWithoutAutoscalers(t *testing.T) {
	report, patches, err := meetAutoscalers(rangedWeb, webManifest)
	require.NoError(t, err)

	require.Equal(t, autoscaling{fallbacks: webTargets}, report)
	require.Len(t, patches, 1)
}

// Ranges the autoscalers apply neither fall back nor pin anything.
func TestMeetAutoscalersWithAutoscalers(t *testing.T) {
	report, patches, err := meetAutoscalers(rangedWeb, webManifest+webAutoscalers)
	require.NoError(t, err)
	require.Equal(t, autoscaling{}, report)
	require.Empty(t, patches)
}

// Fixed values of what the autoscalers scale pin them.
func TestMeetAutoscalersPinAutoscalers(t *testing.T) {
	report, patches, err := meetAutoscalers(fixedWeb, webManifest+webAutoscalers)
	require.NoError(t, err)
	require.Equal(t, autoscaling{pins: webTargets}, report)
	require.Empty(t, patches)
}

// Without requests nothing meets the autoscalers, so the manifests are not even read.
func TestMeetAutoscalersWithoutRequests(t *testing.T) {
	report, patches, err := meetAutoscalers(nil, "kind: [Deployment")
	require.NoError(t, err)
	require.Equal(t, autoscaling{}, report)
	require.Empty(t, patches)
}

// fakeApplication is the application d8-shop.shop.
type fakeApplication struct{}

func (fakeApplication) GetName() string                  { return "d8-shop.shop" }
func (fakeApplication) GetPath() string                  { return "" }
func (fakeApplication) GetRuntimeValues() string         { return "" }
func (fakeApplication) GetValues() addonutils.Values     { return nil }
func (fakeApplication) GetMaintenance() MaintenanceState { return Managed }
func (fakeApplication) GetInstance() string              { return "shop" }
func (fakeApplication) GetPackage() string               { return "shop" }

// gauges returns the labels of every series of the metric set to 1.
func gauges(t *testing.T, storage *metricsstorage.MetricStorage, metric string) []map[string]string {
	t.Helper()

	families, err := storage.Gather()
	require.NoError(t, err)

	var series []map[string]string

	for _, family := range families {
		if family.GetName() != metric {
			continue
		}

		for _, m := range family.GetMetric() {
			require.InDelta(t, 1, m.GetGauge().GetValue(), 0)

			labels := make(map[string]string)
			for _, label := range m.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			series = append(series, labels)
		}
	}

	return series
}

// shopLabels are the labels of a target of web in the application d8-shop.shop.
func shopLabels(container, resourceName, autoscaler string) map[string]string {
	return map[string]string{
		"namespace":  "d8-shop",
		"name":       "shop",
		"kind":       "Deployment",
		"workload":   "web",
		"container":  container,
		"resource":   resourceName,
		"autoscaler": autoscaler,
	}
}

// Every render replaces the autoscaler gauges of the application with its current targets, and
// the release going away drops them. The gauges are registered as deckhouse-controller registers
// them, so the labels published must be the registered ones.
func TestPublishAutoscaling(t *testing.T) {
	storage := metricsstorage.NewMetricStorage(metricsstorage.WithNewRegistry())
	require.NoError(t, metrics.RegisterDeckhouseControllerMetrics(storage))

	service := &Service{metrics: storage}

	service.publishAutoscaling("d8-shop", fakeApplication{}, autoscaling{fallbacks: webTargets, pins: webTargets[1:]})

	require.ElementsMatch(t, []map[string]string{
		shopLabels("", "replicas", "HorizontalPodAutoscaler"),
		shopLabels("app", "cpu", "VerticalPodAutoscaler"),
	}, gauges(t, storage, metrics.ApplicationAutoscalerMissing))
	require.Equal(t, []map[string]string{
		shopLabels("app", "cpu", "VerticalPodAutoscaler"),
	}, gauges(t, storage, metrics.ApplicationAutoscalerPinned))

	service.publishAutoscaling("d8-shop", fakeApplication{}, autoscaling{fallbacks: webTargets[:1]})
	require.Len(t, gauges(t, storage, metrics.ApplicationAutoscalerMissing), 1)
	require.Empty(t, gauges(t, storage, metrics.ApplicationAutoscalerPinned))

	service.publishAutoscaling("d8-shop", fakeApplication{}, autoscaling{fallbacks: webTargets[:1], pins: webTargets})
	require.Len(t, gauges(t, storage, metrics.ApplicationAutoscalerPinned), 2)

	service.expireAutoscaling("d8-shop.shop")
	require.Empty(t, gauges(t, storage, metrics.ApplicationAutoscalerMissing))
	require.Empty(t, gauges(t, storage, metrics.ApplicationAutoscalerPinned))
}
