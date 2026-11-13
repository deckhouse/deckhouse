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

// Package proxyclient reads /upstreams of the local kubernetes-api-proxy,
// probes /readyz of the apiserver and finds the apiserver addresses in the
// EndpointSlice default/kubernetes.
package proxyclient

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/transport"
)

const (
	// ProxyAddr is the data port of the local kubernetes-api-proxy.
	ProxyAddr = "127.0.0.1:6445"
	// upstreamsURL is on the health port of the local kubernetes-api-proxy.
	upstreamsURL = "http://127.0.0.1:6480/upstreams"
	// ProbeTimeout is the timeout of one probe of /readyz, as the default
	// --health-timeout of kubernetes-api-proxy.
	ProbeTimeout = 500 * time.Millisecond

	maxBodyBytes = 1 << 20
)

// Upstreams is the part of /upstreams that the agent uses. The agent does not
// use score, tier and isHealthy.
type Upstreams struct {
	// HasProbe is false for an old proxy: the answer has no probeSeq.
	HasProbe bool
	ProbeSeq uint64
	ProbeOK  bool
	// LastReadyAgo has the lastReadyAgoMs of each upstream that has the field.
	LastReadyAgo []time.Duration
}

type Client struct {
	rest *rest.Config
	// dial is one value for all probes, because the transport cache compares
	// the dial by its pointer.
	dial *transport.DialHolder
	http *http.Client
	url  string
	// factory has one informer: the EndpointSlice default/kubernetes.
	factory informers.SharedInformerFactory
	slices  cache.SharedIndexInformer
}

// New makes a client. cfg gives the TLS settings and the credentials for the
// probes of /readyz.
func New(cfg *rest.Config, k8s kubernetes.Interface) *Client {
	factory := informers.NewSharedInformerFactoryWithOptions(k8s, 0,
		informers.WithNamespace(metav1.NamespaceDefault),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kubernetes").String()
		}),
	)

	return &Client{
		rest:    cfg,
		dial:    &transport.DialHolder{Dial: probeDial},
		http:    &http.Client{Transport: &http.Transport{}},
		url:     upstreamsURL,
		factory: factory,
		slices:  factory.Discovery().V1().EndpointSlices().Informer(),
	}
}

// Run keeps the EndpointSlice informer until ctx ends.
func (c *Client) Run(ctx context.Context) {
	c.factory.Start(ctx.Done())
	<-ctx.Done()
	c.factory.Shutdown()
}

// Upstreams reads /upstreams. An error means that the proxy does not answer.
// Any answer without probeSeq, a 404 or an invalid body included, is an old proxy.
func (c *Client) Upstreams(ctx context.Context) (Upstreams, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return Upstreams{}, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Upstreams{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return Upstreams{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Upstreams{}, nil
	}

	return decode(body), nil
}

func decode(body []byte) Upstreams {
	var v struct {
		ProbeSeq *uint64 `json:"probeSeq"`
		ProbeOK  bool    `json:"probeOK"`
		Nodes    []struct {
			LastReadyAgoMs *int64 `json:"lastReadyAgoMs"`
		} `json:"nodes"`
	}

	if err := json.Unmarshal(body, &v); err != nil || v.ProbeSeq == nil {
		return Upstreams{}
	}

	u := Upstreams{HasProbe: true, ProbeSeq: *v.ProbeSeq, ProbeOK: v.ProbeOK}

	for _, n := range v.Nodes {
		if n.LastReadyAgoMs != nil {
			u.LastReadyAgo = append(u.LastReadyAgo, time.Duration(max(*n.LastReadyAgoMs, 0))*time.Millisecond)
		}
	}

	return u
}

// Ready probes https://addr/readyz and tells whether the answer is 200.
func (c *Client) Ready(ctx context.Context, addr string) bool {
	rt, err := c.transport()
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+addr+"/readyz", nil)
	if err != nil {
		return false
	}

	resp, err := (&http.Client{Transport: rt}).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))

	return resp.StatusCode == http.StatusOK
}

// transport returns the transport of the probes. transport.New keeps it in a
// cache by the TLS settings and the dial, so the probes use one warm
// connection and a new CA file gives a new transport. The dial of the caller
// config is not used.
func (c *Client) transport() (http.RoundTripper, error) {
	tc, err := c.rest.TransportConfig()
	if err != nil {
		return nil, err
	}

	tc.DialHolder = c.dial

	return transport.New(tc)
}

// probeDial sets TCP_USER_TIMEOUT of each probe connection to ProbeTimeout.
// When the network drops packets, the kernel closes the connection after this
// time, and the next probe dials a new connection. Without the option, the
// probes wait on the old connection for the next retransmission, which can
// come some seconds after the network is available again.
func probeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer

	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	_ = setUserTimeout(conn, ProbeTimeout)

	return conn, nil
}

// APIServers returns the host:port of each ready endpoint in the EndpointSlice
// default/kubernetes. The port is the one named https, else the first one,
// else 6443, as in kubernetes-api-proxy.
func (c *Client) APIServers() []string {
	var addrs []string

	for _, obj := range c.slices.GetStore().List() {
		es, ok := obj.(*discoveryv1.EndpointSlice)
		if !ok {
			continue
		}

		port := slicePort(es)

		for _, ep := range es.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}

			for _, a := range ep.Addresses {
				addrs = append(addrs, net.JoinHostPort(a, port))
			}
		}
	}

	return addrs
}

func slicePort(es *discoveryv1.EndpointSlice) string {
	var port int32

	for _, p := range es.Ports {
		if p.Port == nil || *p.Port <= 0 {
			continue
		}

		if p.Name != nil && *p.Name == "https" {
			port = *p.Port

			break
		}

		if port == 0 {
			port = *p.Port
		}
	}

	if port == 0 {
		port = 6443
	}

	return strconv.Itoa(int(port))
}
