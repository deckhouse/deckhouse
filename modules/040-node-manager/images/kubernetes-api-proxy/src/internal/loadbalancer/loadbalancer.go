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

package loadbalancer

import (
	"errors"
	"log/slog"
	"time"

	"github.com/siderolabs/tcpproxy"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/upstream"
)

// Balancer is a health-aware TCP load balancer across a set of upstreams.
//
// A zero-value Balancer is ready to use. Call ServeRoute to bind a listening
// address and wire it to an upstream list. Then start the embedded tcpproxy
// (Start/Run). Close shuts down listeners and health checks.
type Balancer struct {
	tcpproxy.Proxy

	Logger *log.Logger

	MainUpstreamList     *upstream.List
	FallbackUpstreamList *upstream.FallbackList

	DialTimeout     time.Duration
	KeepAlivePeriod time.Duration
	TCPUserTimeout  time.Duration
}

// IsHealthy reports whether at least one upstream is currently available
// for the configured route.
func (b *Balancer) IsHealthy() (bool, error) {
	if b.MainUpstreamList == nil {
		return false, nil
	}

	_, err := b.MainUpstreamList.Pick()
	if err != nil {
		if errors.Is(err, upstream.ErrNoUpstreams) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (b *Balancer) ExportNodes() ([]upstream.ExportNode, error) {
	if b.MainUpstreamList == nil {
		return nil, errors.New("main upstream list is empty")
	}

	return b.MainUpstreamList.ExportNodes()
}

// ServeRoute installs a load balancer route from the listening ip:port to the
// provided upstream addresses.
//
// Background health checks run automatically and only healthy upstreams are
// picked for proxying. Call before Start.
func (b *Balancer) ServeRoute(ipPort string) error {
	if b.Logger != nil {
		b.Logger.Debug("adding route",
			slog.String("route", ipPort),
		)
	}

	b.Proxy.AddRoute(ipPort, &lbTarget{
		list:            b.MainUpstreamList,
		fallbackList:    b.FallbackUpstreamList,
		logger:          b.Logger,
		route:           ipPort,
		dialTimeout:     b.DialTimeout,
		keepAlivePeriod: b.KeepAlivePeriod,
		tcpUserTimeout:  b.TCPUserTimeout,
	})

	if b.Logger != nil {
		b.Logger.Debug("route added",
			slog.String("route", ipPort),
		)
	}

	return nil
}

// Close shuts down the proxy listeners and stops health checks.
func (b *Balancer) Close() error {
	if err := b.Proxy.Close(); err != nil {
		return err
	}

	if b.MainUpstreamList != nil {
		b.MainUpstreamList.Shutdown()
	}
	if b.FallbackUpstreamList != nil {
		b.FallbackUpstreamList.Shutdown()
	}

	return nil
}
