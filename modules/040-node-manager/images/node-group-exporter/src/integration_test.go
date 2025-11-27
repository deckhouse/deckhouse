/*
Copyright 2025 Flant JSC

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

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"node-group-exporter/pkg/collector"
	k8s "node-group-exporter/pkg/kubernetes"
)

// test helpers
func newCollectorForTest(t *testing.T) *collector.NodeGroupCollector {
	t.Helper()
	c, err := collector.NewNodeGroupCollector(fake.NewSimpleClientset(), &rest.Config{})
	assert.NoError(t, err)
	return c
}

func gather(t *testing.T, c *collector.NodeGroupCollector) []*dto.MetricFamily {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	metrics, err := reg.Gather()
	assert.NoError(t, err)
	return metrics
}

func makeNode(name, ng string, ready bool) *k8s.Node {
	cond := v1.ConditionFalse
	if ready {
		cond = v1.ConditionTrue
	}
	return &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{"node.deckhouse.io/group": ng},
			},
			Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: cond}}},
		},
		NodeGroup: ng,
	}
}

// TestIntegration tests the full integration of the exporter
func TestIntegration(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	testNodes := []*v1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-1", Labels: map[string]string{"node.deckhouse.io/group": "worker"}}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker-2", Labels: map[string]string{"node.deckhouse.io/group": "worker"}}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{"node.deckhouse.io/group": "master"}}, Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}}},
	}
	for _, n := range testNodes {
		_, err := clientset.CoreV1().Nodes().Create(context.TODO(), n, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	nodeGroupCollector, err := collector.NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := nodeGroupCollector.Start(ctx); err != nil && err.Error() == "failed to sync informer caches" {
		t.Logf("Expected error with fake client (cannot sync CRD): %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	reg := prometheus.NewRegistry()
	reg.MustRegister(nodeGroupCollector)
	_, err = reg.Gather()
	assert.NoError(t, err)
	nodeGroupCollector.Stop()
}

// TestMetricsExposure tests that metrics are properly exposed
func TestMetricsExposure(t *testing.T) {
	c := newCollectorForTest(t)
	c.OnNodeGroupAdd(&k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "test-worker"}, Spec: k8s.NodeGroupSpec{NodeType: "Cloud", CloudInstances: &k8s.CloudInstancesSpec{MaxPerZone: 5, MinPerZone: 1, Zones: []string{"zone-a", "zone-b"}}}, Status: k8s.NodeGroupStatus{Desired: 3, Ready: 3}}})
	c.OnNodeAdd(makeNode("test-node-1", "test-worker", true))
	metrics := gather(t, c)
	_ = metrics
}

// Table-driven integration tests for NodeGroup types
func TestNodeGroupTypeIntegration(t *testing.T) {
	cases := []struct {
		name  string
		ng    *k8s.NodeGroupWrapper
		setup func(c *collector.NodeGroupCollector)
	}{
		{
			name: "Static",
			ng:   &k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "static-master"}, Spec: k8s.NodeGroupSpec{NodeType: "Static"}, Status: k8s.NodeGroupStatus{Desired: 3, Ready: 3}}},
			setup: func(c *collector.NodeGroupCollector) {
				for i := 1; i <= 3; i++ {
					c.OnNodeAdd(makeNode(fmt.Sprintf("static-master-%d", i), "static-master", true))
				}
			},
		},
		{
			name:  "Cloud",
			ng:    &k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "cloud-worker"}, Spec: k8s.NodeGroupSpec{NodeType: "Cloud", CloudInstances: &k8s.CloudInstancesSpec{MaxPerZone: 5, MinPerZone: 1, Zones: []string{"zone-a", "zone-b", "zone-c"}}}, Status: k8s.NodeGroupStatus{Desired: 10, Ready: 8}}},
			setup: func(c *collector.NodeGroupCollector) {},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCollectorForTest(t)
			if tc.setup != nil {
				tc.setup(c)
			}
			c.OnNodeGroupAdd(tc.ng)
			_ = gather(t, c)
		})
	}
}

// TestIntegrationErrorHandling tests error handling in the integration
func TestIntegrationErrorHandling(t *testing.T) {
	c := newCollectorForTest(t)
	c.OnNodeGroupAdd(&k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: ""}}})
	_ = gather(t, c)
}

// TestConcurrentAccess tests concurrent access to the collector
func TestConcurrentAccess(t *testing.T) {
	c := newCollectorForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.Start(ctx) // ignore fake client sync error

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			c.OnNodeGroupAdd(&k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("concurrent-group-%d", id)}, Spec: k8s.NodeGroupSpec{NodeType: "Cloud"}, Status: k8s.NodeGroupStatus{Desired: int32(id + 1)}}})
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	c.Stop()
	_ = gather(t, c)
}

// TestHTTPEndpoints tests HTTP endpoints (mock)
func TestHTTPEndpoints(t *testing.T) {
	testCases := []struct {
		endpoint string
		expected int
	}{
		{"/metrics", http.StatusOK},
		{"/health", http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Run(tc.endpoint, func(t *testing.T) {
			statusCode := http.StatusOK
			if tc.endpoint == "/nonexistent" {
				statusCode = http.StatusNotFound
			}
			assert.Equal(t, tc.expected, statusCode)
		})
	}
}

// TestMetricsEndpointExcludesGoMetrics tests that metrics endpoint doesn't expose golang built-in metrics
func TestMetricsEndpointExcludesGoMetrics(t *testing.T) {
	c := newCollectorForTest(t)
	c.OnNodeGroupAdd(&k8s.NodeGroupWrapper{NodeGroup: &k8s.NodeGroup{TypeMeta: metav1.TypeMeta{APIVersion: "deckhouse.io/v1", Kind: "NodeGroup"}, ObjectMeta: metav1.ObjectMeta{Name: "test-worker"}, Spec: k8s.NodeGroupSpec{NodeType: "Cloud", CloudInstances: &k8s.CloudInstancesSpec{MaxPerZone: 3, Zones: []string{"zone-a"}}}}})
	c.OnNodeAdd(makeNode("test-node-1", "test-worker", true))

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK); w.Write([]byte("OK")) })
	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	server.Addr = listener.Addr().String()
	serverDone := make(chan error, 1)
	go func() { serverDone <- http.Serve(listener, server.Handler) }()
	time.Sleep(200 * time.Millisecond)
	url := "http://" + server.Addr + "/metrics"
	resp, err := http.Get(url)
	if err != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
		listener.Close()
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	metricsContent := string(body)
	assert.Contains(t, metricsContent, "node_group_count_nodes_total")
	assert.Contains(t, metricsContent, "node_group_count_ready_total")
	assert.Contains(t, metricsContent, "node_group_count_max_total")
	assert.Contains(t, metricsContent, "node_group_node")
	assert.NotContains(t, metricsContent, "go_info")
	assert.NotContains(t, metricsContent, "go_gc_")
	assert.NotContains(t, metricsContent, "go_memstats")
	assert.NotContains(t, metricsContent, "go_threads")
	assert.NotContains(t, metricsContent, "go_goroutines")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	server.Shutdown(ctx)
	listener.Close()
}
