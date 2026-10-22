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
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/distribution/distribution/v3/configuration"
)

func debugSettings(addr string) *configuration.Configuration {
	settings := &configuration.Configuration{}
	settings.HTTP.Debug.Addr = addr
	settings.HTTP.Debug.Prometheus.Enabled = true
	settings.HTTP.Debug.Prometheus.Path = "/metrics"
	return settings
}

// The listener the configuration names answers: the metrics the sidecar scrapes, and upstream's
// /debug/health. Neither did while the wrapper left the listener to a command it never runs.
func TestDebugListenerServesMetricsAndHealth(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	addr, err := debugListener(debugSettings("127.0.0.1:0"), log)
	if err != nil {
		t.Fatalf("the debug listener did not open: %v", err)
	}

	for _, path := range []string{"/metrics", "/debug/health"} {
		response, err := http.Get("http://" + addr.String() + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Errorf("GET %s answered %d, want 200", path, response.StatusCode)
		}
	}
}

// A port someone else holds is reported, not fatal, and registering the handlers again does not panic.
func TestDebugListenerOnABusyPortIsNotFatal(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	held, err := debugListener(debugSettings("127.0.0.1:0"), log)
	if err != nil {
		t.Fatalf("the first listener did not open: %v", err)
	}

	if _, err := debugListener(debugSettings(held.String()), log); err == nil {
		t.Fatal("a second listener on a held port reported no error")
	}
}

func TestDebugListenerWithoutAnAddressOpensNothing(t *testing.T) {
	addr, err := debugListener(&configuration.Configuration{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if addr != nil || err != nil {
		t.Fatalf("got %v, %v; want nothing opened and no error", addr, err)
	}
}
