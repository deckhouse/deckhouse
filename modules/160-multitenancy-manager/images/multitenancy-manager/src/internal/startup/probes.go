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

package startup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Probe paths of the manager (controller-runtime's defaults), answered in its place while it is not
// started yet.
const (
	LivenessPath  = "/healthz"
	ReadinessPath = "/readyz"
)

const (
	probeReadHeaderTimeout = 5 * time.Second
	probeShutdownTimeout   = 5 * time.Second
)

// ServeProbesWhile answers the probes of the kubelet on listener while wait runs, before the manager
// is created: the liveness path with 200, so the kubelet does not restart a container that only
// waits, and the readiness path with 503, so the pod gets no traffic. When wait returns, the server
// is shut down and listener closed, so the manager can bind the same address, and the error of wait
// is returned.
func ServeProbesWhile(ctx context.Context, listener net.Listener, wait func(context.Context) error) error {
	mux := http.NewServeMux()
	mux.HandleFunc(LivenessPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	})
	mux.HandleFunc(ReadinessPath, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "the controller has not started yet", http.StatusServiceUnavailable)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: probeReadHeaderTimeout}

	served := make(chan error, 1)
	go func() {
		served <- server.Serve(listener)
	}()

	waitErr := wait(ctx)

	// Shutdown closes the listener; the probes are cheap, so a short grace for the ones in flight is
	// enough. The context of the wait may be done already, the shutdown must not depend on it.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), probeShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve the probes: %w", err))
	}

	if waitErr != nil {
		return waitErr
	}
	if shutdownErr != nil {
		return fmt.Errorf("stop serving the probes: %w", shutdownErr)
	}
	return nil
}
