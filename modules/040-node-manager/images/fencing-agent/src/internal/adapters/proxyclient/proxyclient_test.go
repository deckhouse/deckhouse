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

package proxyclient

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Upstreams
	}{
		{
			// etcd without quorum: isHealthy, score and tier say "healthy",
			// but no upstream was ready for 5 s. Only lastReadyAgoMs counts.
			name: "nobodyReady trap",
			body: `{"isHealthy":true,"probeSeq":1200,"probeOK":false,"nodes":[` +
				`{"upstream":"10.0.0.1:6443","score":3,"tier":0,"lastReadyAgoMs":5000},` +
				`{"upstream":"10.0.0.2:6443","score":3,"tier":0,"lastReadyAgoMs":5100},` +
				`{"upstream":"10.0.0.3:6443","score":3,"tier":0}]}`,
			want: Upstreams{HasProbe: true, ProbeSeq: 1200, LastReadyAgo: []time.Duration{5 * time.Second, 5100 * time.Millisecond}},
		},
		{
			name: "one upstream ready",
			body: `{"isHealthy":false,"probeSeq":7,"probeOK":true,"nodes":[{"lastReadyAgoMs":800},{},{}]}`,
			want: Upstreams{HasProbe: true, ProbeSeq: 7, ProbeOK: true, LastReadyAgo: []time.Duration{800 * time.Millisecond}},
		},
		{
			name: "proxy restart: empty list",
			body: `{"isHealthy":false,"probeSeq":9,"probeOK":false,"nodes":[]}`,
			want: Upstreams{HasProbe: true, ProbeSeq: 9},
		},
		{
			name: "old proxy: no probeSeq",
			body: `{"isHealthy":true,"nodes":[{"score":3,"tier":0,"lastReadyAgoMs":10}]}`,
			want: Upstreams{},
		},
		{
			name: "invalid JSON",
			body: `{"probeSeq":`,
			want: Upstreams{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decode([]byte(tt.body))
			if got.HasProbe != tt.want.HasProbe || got.ProbeSeq != tt.want.ProbeSeq || got.ProbeOK != tt.want.ProbeOK ||
				!slices.Equal(got.LastReadyAgo, tt.want.LastReadyAgo) {
				t.Fatalf("decode() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUpstreams(t *testing.T) {
	status, body := http.StatusOK, `{"probeSeq":5,"probeOK":true,"nodes":[{"lastReadyAgoMs":100}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upstreams" {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := New(&rest.Config{}, fake.NewClientset())
	c.url = srv.URL + "/upstreams"

	u, err := c.Upstreams(t.Context())
	if err != nil || !u.HasProbe || u.ProbeSeq != 5 || !u.ProbeOK {
		t.Fatalf("new proxy: got %+v, %v", u, err)
	}

	status = http.StatusNotFound
	if u, err = c.Upstreams(t.Context()); err != nil || u.HasProbe {
		t.Fatalf("404 must be an old proxy without an error: got %+v, %v", u, err)
	}

	srv.Close()

	if _, err = c.Upstreams(t.Context()); err == nil {
		t.Fatal("a proxy that does not answer must give an error")
	}
}

func TestReady(t *testing.T) {
	status := http.StatusOK

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(status)
	}))
	defer srv.Close()

	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	c := New(&rest.Config{TLSClientConfig: rest.TLSClientConfig{CAData: ca}}, fake.NewClientset())
	addr := srv.Listener.Addr().String()

	if !c.Ready(t.Context(), addr) {
		t.Fatal("200 must be ready")
	}

	status = http.StatusInternalServerError
	if c.Ready(t.Context(), addr) {
		t.Fatal("500 must be not ready")
	}

	srv.Close()

	if c.Ready(t.Context(), addr) {
		t.Fatal("no answer must be not ready")
	}
}

// A probe must not make a new transport each time, also when the config has a custom dial.
func TestProbesShareOneTransport(t *testing.T) {
	var conns atomic.Int32

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.StartTLS()
	defer srv.Close()

	cfg := &rest.Config{
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		Dial:            (&net.Dialer{}).DialContext,
	}
	c := New(cfg, fake.NewClientset())

	first, err := c.transport()
	if err != nil {
		t.Fatal(err)
	}

	second, err := c.transport()
	if err != nil {
		t.Fatal(err)
	}

	if first != second {
		t.Fatal("the probes must use one transport")
	}

	for range 20 {
		if !c.Ready(t.Context(), srv.Listener.Addr().String()) {
			t.Fatal("200 must be ready")
		}
	}

	if got := conns.Load(); got != 1 {
		t.Fatalf("server connections = %d, want 1", got)
	}

	if cfg.Dial == nil {
		t.Fatal("New must not change the config of the caller")
	}
}

func TestAPIServers(t *testing.T) {
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta:  metav1.ObjectMeta{Name: "kubernetes", Namespace: metav1.NamespaceDefault},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports: []discoveryv1.EndpointPort{
			{Name: ptrTo("metrics"), Port: ptrTo[int32](9000)},
			{Name: ptrTo("https"), Port: ptrTo[int32](6443)},
		},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptrTo(true)}},
			{Addresses: []string{"10.0.0.3"}, Conditions: discoveryv1.EndpointConditions{Ready: ptrTo(false)}},
		},
	}

	c := New(&rest.Config{}, fake.NewClientset(slice))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)
		c.Run(ctx)
	}()

	defer func() {
		cancel()
		<-done
	}()

	if !cache.WaitForCacheSync(ctx.Done(), c.slices.HasSynced) {
		t.Fatal("informer did not sync")
	}

	want := []string{"10.0.0.1:6443", "10.0.0.2:6443"}
	if got := c.APIServers(); !slices.Equal(got, want) {
		t.Fatalf("APIServers() = %v, want %v", got, want)
	}
}

func TestSlicePort(t *testing.T) {
	tests := []struct {
		name  string
		ports []discoveryv1.EndpointPort
		want  string
	}{
		{"https wins", []discoveryv1.EndpointPort{{Port: ptrTo[int32](1)}, {Name: ptrTo("https"), Port: ptrTo[int32](2)}}, "2"},
		{"first port", []discoveryv1.EndpointPort{{Name: ptrTo("a"), Port: ptrTo[int32](7)}, {Port: ptrTo[int32](8)}}, "7"},
		{"default", nil, "6443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := slicePort(&discoveryv1.EndpointSlice{Ports: tt.ports}); got != tt.want {
				t.Fatalf("slicePort() = %s, want %s", got, tt.want)
			}
		})
	}
}

func ptrTo[T any](v T) *T { return &v }
