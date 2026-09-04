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

// Package serve runs the registry: one process, one store, and up to three listeners.
//
// The serving one is the upstream project's own — `registry.NewRegistry` and its `ListenAndServe`,
// with TLS, the debug and metrics listener, tracing and the graceful drain exactly as upstream
// wrote them. What this module adds is attached through upstream's own extension point,
// `registry.RegisterHandler`: the token proxy and the decision about whose word on a client's
// address is taken.
//
// The second is the write endpoint, and it exists because the first one is a pull-through cache.
// Upstream's proxy stores answer every write with UNSUPPORTED — deliberately, and measured here —
// while the store this module runs is filled BY writes for as long as it still has an upstream to
// serve from. So a second app over the same directory, built from the same configuration with the
// proxy section removed, serves the pushes. It is hand-built rather than a second
// `registry.NewRegistry`, because health checks are registered in a process-global registry and a
// second registration panics.
//
// The third is the loopback rewriter, which the cache treats as its upstream — see the upstream
// package. Nothing outside this process can reach it.
package serve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/health"
	dregistry "github.com/distribution/distribution/v3/registry"
	"github.com/distribution/distribution/v3/registry/handlers"
	gorhandlers "github.com/gorilla/handlers"

	"github.com/deckhouse/registry-distribution/internal/authproxy"
	"github.com/deckhouse/registry-distribution/internal/config"
	"github.com/deckhouse/registry-distribution/internal/realip"
	"github.com/deckhouse/registry-distribution/internal/upstream"
)

// LoopbackUpstreamAddress is where the rewriter listens, and therefore what the cache is pointed at.
//
// A constant of this binary rather than a rendered value: the two halves are one process, nothing
// else may reach the address, and a port in a configuration file is one more thing that can
// disagree with itself. The cache's `proxy.remoteurl` is set from here, which is also why the
// rendered configuration carries no proxy section at all.
const LoopbackUpstreamAddress = "127.0.0.1:5004"

// Run starts everything and returns when the serving listener stops.
func Run(ctx context.Context, distribution *configuration.Configuration, wrapper *config.Wrapper, log *slog.Logger) error {
	if distribution.Proxy.RemoteURL != "" {
		// The rendered configuration must not carry one: which upstream a miss goes to, and under
		// which path, is decided here — see the upstream package — and a second answer in the file
		// would be a second opinion nobody reconciles.
		return errors.New("the rendered configuration carries a proxy section; the upstream is " +
			"configured in this module's own file instead")
	}

	rewriter, err := upstream.New(wrapper, LoopbackUpstreamAddress, log)
	if err != nil {
		return err
	}

	if rewriter != nil {
		listener, err := net.Listen("tcp", rewriter.Address)
		if err != nil {
			return fmt.Errorf("listening for the loopback upstream on %s: %w", rewriter.Address, err)
		}
		go func() {
			if err := rewriter.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				// Not fatal on its own: what it costs is cache misses, and the store keeps serving
				// everything it holds. A cluster that is already air-gapped does not notice at all.
				log.Error("the loopback upstream stopped", "error", err.Error())
			}
		}()

		// Never expires. The store holds what the cluster needs to pull while the upstream is
		// unreachable, so nothing in it may be evicted on a timer — upstream reads a zero TTL as
		// exactly that, which is what replaced this module's own patches.
		never := time.Duration(0)
		distribution.Proxy = configuration.Proxy{RemoteURL: "http://" + rewriter.Address, TTL: &never}
		log.Info("the cache will fetch misses through the loopback upstream",
			"upstream", wrapper.Upstream.Address, "path", wrapper.Upstream.Path)
	} else {
		log.Info("no upstream is configured, so the store is authoritative: " +
			"it serves what it holds and nothing else")
	}

	// What this module adds to the serving listener, through upstream's own extension point. The
	// serving listener has nothing in front of it, so no client certificate authority is given to
	// realip: a header there is a claim a node made about itself.
	dregistry.RegisterHandler(func(_ *configuration.Configuration, handler http.Handler) http.Handler {
		wrapped, err := authproxy.Handler(wrapper.AuthProxy, handler)
		if err != nil {
			log.Error("the token proxy could not be mounted", "error", err.Error())
			return handler
		}
		return wrapped
	})
	dregistry.RegisterHandler(func(_ *configuration.Configuration, handler http.Handler) http.Handler {
		wrapped, err := realip.Handler("", handler)
		if err != nil {
			log.Error("the client address handling could not be mounted", "error", err.Error())
			return handler
		}
		return wrapped
	})

	serving, err := dregistry.NewRegistry(ctx, distribution)
	if err != nil {
		return fmt.Errorf("building the serving registry: %w", err)
	}

	if wrapper.WriteEndpoint.Address != "" {
		writing, err := writeEndpoint(ctx, distribution, wrapper, log)
		if err != nil {
			return err
		}
		go func() {
			if err := writing(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				// Fatal for the pushes, not for the pulls. Reported loudly and left to the
				// kubelet: a store that cannot be written is still a store the cluster pulls from,
				// and taking the serving half down with it would turn a failed push into an outage.
				log.Error("the write endpoint stopped", "error", err.Error())
			}
		}()
	}

	return serving.ListenAndServe()
}

// writeEndpoint builds the listener that accepts a push, and returns the function that runs it.
func writeEndpoint(
	ctx context.Context, serving *configuration.Configuration, wrapper *config.Wrapper, log *slog.Logger,
) (func() error, error) {
	if wrapper.WriteEndpoint.Address == serving.HTTP.Addr {
		return nil, fmt.Errorf(
			"the write endpoint and the serving endpoint are both on %s, and they are two "+
				"listeners in one process", serving.HTTP.Addr)
	}

	settings := writeConfiguration(serving, wrapper)

	app := handlers.NewApp(ctx, settings)
	// No RegisterHealthChecks: the health registry is global to the process and the serving half
	// has already registered the checks for this storage. A second registration panics, and there
	// is nothing it would watch that the first does not.

	var handler http.Handler = app
	handler = health.Handler(handler)

	handler, err := authproxy.Handler(wrapper.AuthProxy, handler)
	if err != nil {
		return nil, err
	}
	// The one listener an ingress fronts, and therefore the one where a forwarded address can be
	// worth believing — if it comes from behind the certificate this module issued to that ingress.
	handler, err = realip.Handler(wrapper.WriteEndpoint.ClientCertCA, handler)
	if err != nil {
		return nil, err
	}
	if !settings.Log.AccessLog.Disabled {
		handler = gorhandlers.CombinedLoggingHandler(os.Stdout, handler)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listener, err := net.Listen("tcp", wrapper.WriteEndpoint.Address)
	if err != nil {
		return nil, fmt.Errorf("listening for the write endpoint on %s: %w",
			wrapper.WriteEndpoint.Address, err)
	}

	if settings.HTTP.TLS.Certificate != "" {
		tlsConfig, err := writeTLS(settings, wrapper)
		if err != nil {
			return nil, err
		}
		listener = tls.NewListener(listener, tlsConfig)
	}

	log.Info("the write endpoint accepts pushes", "address", wrapper.WriteEndpoint.Address)

	return func() error { return server.Serve(listener) }, nil
}

// writeConfiguration derives the write endpoint's configuration from the serving one.
//
// Derived rather than rendered separately, and that is the point of running both in one process:
// authentication, the token service, the storage path and the read-only flag a garbage collection
// sets cannot drift between the two halves, because there is only one of each. What differs is
// exactly what has to: no proxying, so a push is answered by the store; its own address, since two
// listeners cannot share a port; and no debug listener, because the metrics collectors are
// registered process-globally and a second registration panics.
func writeConfiguration(serving *configuration.Configuration, wrapper *config.Wrapper) *configuration.Configuration {
	settings := *serving

	// The storage parameters are a map, so a shallow copy would share them with the half that
	// serves reads.
	settings.Storage = configuration.Storage{}
	for kind, parameters := range serving.Storage {
		copied := configuration.Parameters{}
		for key, value := range parameters {
			copied[key] = value
		}
		settings.Storage[kind] = copied
	}

	settings.Proxy = configuration.Proxy{}
	settings.HTTP.Addr = wrapper.WriteEndpoint.Address
	settings.HTTP.Debug.Addr = ""
	settings.HTTP.Debug.Prometheus.Enabled = false

	return &settings
}

// writeTLS is the serving certificate plus the authority whose client certificates this listener
// asks for.
//
// Asks for, not requires: a push that presents no certificate is served, it simply does not get to
// say which address it came from. Requiring one would mean every client of the publication endpoint
// needs a certificate this module does not issue to anybody but the ingress.
func writeTLS(settings *configuration.Configuration, wrapper *config.Wrapper) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(settings.HTTP.TLS.Certificate, settings.HTTP.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("loading the write endpoint's certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.NoClientCert,
	}

	if wrapper.WriteEndpoint.ClientCertCA != "" {
		pem, err := os.ReadFile(wrapper.WriteEndpoint.ClientCertCA)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", wrapper.WriteEndpoint.ClientCertCA, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s holds no certificate this can use",
				wrapper.WriteEndpoint.ClientCertCA)
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequestClientCert
	}

	return tlsConfig, nil
}
