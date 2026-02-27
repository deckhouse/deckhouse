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
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/siderolabs/tcpproxy"

	"github.com/deckhouse/deckhouse/pkg/log"

	"kubernetes-api-proxy/internal/upstream"
)

// lbTarget implements tcpproxy.Target and proxies connections to a picked
// upstream chosen from the upstream.List based on current health and tiering.
type lbTarget struct {
	list         *upstream.List
	fallbackList *upstream.FallbackList

	logger          *log.Logger
	route           string
	dialTimeout     time.Duration
	keepAlivePeriod time.Duration
	tcpUserTimeout  time.Duration
}

// HandleConn picks an upstream and proxies the connection to it. If dialing the
// upstream fails, the backend is marked as down and the client connection is
// closed.
func (t *lbTarget) HandleConn(src net.Conn) {
	var backendAddr string

	// 1. Try to pick address from MainList (served with endpoint slices)
	backend, err := t.list.Pick()
	if err == nil {
		backendAddr = backend.Address()
	}

	// 2. If we don't get any, try to get it from fallback list
	if backendAddr == "" {
		if fallbackBackend, err := t.fallbackList.Pick(); err == nil {
			backendAddr = fallbackBackend.Address()
		}
	}

	// 3. If fallback list is empty for any reason - trying to serve with default kubernetes service host:port
	if backendAddr == "" {
		t.logger.Warn(
			"failed to pick upstream from fallback list, " +
				"falling to KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT",
		)

		kubernetesHost := os.Getenv("KUBERNETES_SERVICE_HOST")
		kubernetesPort := os.Getenv("KUBERNETES_SERVICE_PORT")

		backendAddr = fmt.Sprintf("%s:%s", kubernetesHost, kubernetesPort)
	}

	// 4. If any of steps don't give us backendAddr, fail to serve, close connection and give up
	if backendAddr == "" {
		_ = src.Close()

		if t.logger != nil {
			t.logger.Error("no upstreams available to handle connection")
		}

		return
	}

	if t.logger != nil {
		t.logger.Debug(
			"proxying connection",
			slog.String("route", t.route),
			slog.String("upstream", backendAddr),
			slog.String("client", src.RemoteAddr().String()),
		)
	}

	proxy := &tcpproxy.DialProxy{
		Addr:            backendAddr,
		KeepAlivePeriod: t.keepAlivePeriod,
		DialTimeout:     t.dialTimeout,
		TCPUserTimeout:  t.tcpUserTimeout,
		OnDialError: func(src net.Conn, dstDialErr error) {
			if backend != nil {
				t.list.Down(backend)
			}

			if t.logger != nil {
				t.logger.Warn(
					"failed to dial upstream",
					slog.String("addr", backendAddr),
					slog.String("error", dstDialErr.Error()),
				)
			}

			_ = src.Close()
		},
	}

	proxy.HandleConn(src)
}
