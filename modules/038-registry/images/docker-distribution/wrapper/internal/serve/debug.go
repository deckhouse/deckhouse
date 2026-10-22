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

package serve

import (
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/distribution/distribution/v3/configuration"
	metrics "github.com/docker/go-metrics"
)

// debugOnce keeps the metrics handler to one registration per process: the default mux panics on a
// second, and the tests start more than one registry in the same binary.
var debugOnce sync.Once

// debugListener serves `http.debug.addr`: the metrics, and upstream's /debug/health.
//
// Upstream starts this listener from its `serve` command, not from registry.NewRegistry, and the
// wrapper never runs that command — it builds the registry itself. So the listener the configuration
// names was never opened: the metrics sidecar scraped a port nobody listened on, and a readiness probe
// pointed there never passed.
//
// The one difference from upstream is what a listener that cannot open does. Upstream exits the
// process; here it is reported and the registry keeps serving. Metrics are not worth an outage, and
// the port is not this registry's alone on a host-networked node: the previous implementation's
// in-cluster proxy uses it too, while a cluster moves between the two.
func debugListener(settings *configuration.Configuration, log *slog.Logger) (net.Addr, error) {
	addr := settings.HTTP.Debug.Addr
	if addr == "" {
		return nil, nil
	}

	debugOnce.Do(func() {
		if settings.HTTP.Debug.Prometheus.Enabled {
			path := settings.HTTP.Debug.Prometheus.Path
			if path == "" {
				path = "/metrics"
			}
			http.Handle(path, metrics.Handler())
		}
	})

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("the debug listener could not open; the registry serves without metrics", "address", addr,
			"error", err.Error())
		return nil, err
	}
	go func() {
		// The default mux: /debug/health is registered there by upstream's health package, and the
		// metrics handler above.
		if err := http.Serve(listener, nil); err != nil {
			log.Error("the debug listener stopped", "address", addr, "error", err.Error())
		}
	}()
	return listener.Addr(), nil
}
