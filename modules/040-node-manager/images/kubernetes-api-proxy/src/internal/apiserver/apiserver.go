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

package apiserver

import (
	"errors"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/loadbalancer"
	"kubernetes-api-proxy/internal/upstream"
)

// LoadBalancer provides a TCP load balancer for the Kubernetes apiserver
// with a way to update the set of upstream endpoints at runtime.
//
// The data path is implemented by the loadbalancer package (tcpproxy-based),
// while upstream health is tracked via HTTP /readyz probes.
type LoadBalancer struct {
	lb       loadbalancer.Balancer
	endpoint string
}

// LoadBalancerOption configures the load balancer settings.
type LoadBalancerOption func(*LoadBalancer)

// WithDialTimeout sets the TCP dial timeout for establishing connections to
// upstream API servers.
func WithDialTimeout(timeout time.Duration) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.lb.DialTimeout = timeout
	}
}

// WithKeepAlivePeriod sets the TCP keepalive period for both client and
// upstream connections.
func WithKeepAlivePeriod(period time.Duration) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.lb.KeepAlivePeriod = period
	}
}

// WithTCPUserTimeout sets the TCP_USER_TIMEOUT (Linux only). If a connection
// has unacknowledged data for longer than this duration, the connection is
// closed by the kernel.
func WithTCPUserTimeout(timeout time.Duration) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.lb.TCPUserTimeout = timeout
	}
}

func WithMainUpstreamList(list *upstream.List) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.lb.MainUpstreamList = list
	}
}

func WithFallbackUpstreamList(list *upstream.FallbackList) LoadBalancerOption {
	return func(lb *LoadBalancer) {
		lb.lb.FallbackUpstreamList = list
	}
}

// NewLoadBalancer initializes the load balancer at given address:port.
// The upstream list starts empty and will be provided via discovery.
//
// If bindPort is zero, this function returns an error.
func NewLoadBalancer(
	bindAddress string,
	bindPort int,
	logger *log.Logger,
	options ...LoadBalancerOption,
) (*LoadBalancer, error) {
	if bindPort == 0 {
		return nil, errors.New("bindPort must be set")
	}

	lb := &LoadBalancer{
		endpoint: net.JoinHostPort(bindAddress, strconv.Itoa(bindPort)),
	}

	// set aggressive timeouts to prevent proxying to unhealthy upstreams
	lb.lb.DialTimeout = time.Second
	lb.lb.KeepAlivePeriod = time.Second
	lb.lb.TCPUserTimeout = time.Second

	for _, option := range options {
		option(lb)
	}

	lb.lb.Logger = logger.
		WithGroup("apiserver").
		With(slog.String("endpoint", lb.endpoint))

	if err := lb.lb.ServeRoute(lb.endpoint); err != nil {
		return nil, err
	}

	return lb, nil
}

// Endpoint returns the listening endpoint in "host:port" form.
func (lb *LoadBalancer) Endpoint() string {
	return lb.endpoint
}

// Healthy reports whether at least one upstream is currently considered
// available by the health checker.
func (lb *LoadBalancer) Healthy() (bool, error) {
	ok, err := lb.lb.IsHealthy()
	if lb.lb.Logger != nil {
		if err != nil {
			lb.lb.Logger.Debug("health check error",
				slog.String("error", err.Error()),
			)
		} else {
			lb.lb.Logger.Debug("health status",
				slog.Bool("healthy", ok),
			)
		}
	}
	return ok, err
}

func (lb *LoadBalancer) Nodes() ([]upstream.ExportNode, error) {
	return lb.lb.MainUpstreamList.ExportNodes()
}

func (lb *LoadBalancer) Start() error {
	return lb.lb.Start()
}

// Shutdown stops the listener, terminates health checks, and waits for
// in-flight connections to drain.
func (lb *LoadBalancer) Shutdown() error {
	if err := lb.lb.Close(); err != nil {
		return err
	}

	lb.lb.Wait() //nolint:errcheck

	if lb.lb.Logger != nil {
		lb.lb.Logger.Debug("load balancer shutdown complete")
	}

	return nil
}
