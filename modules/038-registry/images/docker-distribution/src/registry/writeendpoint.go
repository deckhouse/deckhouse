package registry

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/docker/distribution/configuration"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/health"
	"github.com/docker/distribution/registry/handlers"
	gorhandlers "github.com/gorilla/handlers"
)

// newWriteEndpoint builds the half of this registry that accepts pushes: the same storage, served on
// its own address with no proxying. See configuration.WriteEndpoint for why it exists.
//
// Nil when no address is configured, which is every registry that is not a cache — such a registry
// already accepts writes on its only listener.
func newWriteEndpoint(ctx context.Context, config *configuration.Configuration) (*handlers.App, *http.Server, error) {
	if config.WriteEndpoint.Addr == "" {
		return nil, nil, nil
	}
	if config.WriteEndpoint.Addr == config.HTTP.Addr {
		return nil, nil, fmt.Errorf(
			"the write endpoint and the serving endpoint are both configured on %v, and they are two listeners in one process",
			config.HTTP.Addr)
	}

	writeConfig := writeEndpointConfiguration(config)

	app := handlers.NewApp(ctx, writeConfig)
	// No RegisterHealthChecks: the health registry is global to the process, and the serving half has
	// already registered the checks for this storage. Registering them again panics on the duplicate,
	// and there is nothing a second registration would watch that the first does not.

	handler := alive("/", app)
	handler = health.Handler(handler)
	handler = authProxyHandler(ctx, writeConfig, handler)
	handler = proxyHeadersHandler(ctx, writeConfig, handler)
	handler = panicHandler(handler)
	if !writeConfig.Log.AccessLog.Disabled {
		handler = gorhandlers.CombinedLoggingHandler(os.Stdout, handler)
	}

	dcontext.GetLogger(ctx).Infof("write endpoint configured on %v", config.WriteEndpoint.Addr)

	return app, &http.Server{Handler: handler}, nil
}

// writeEndpointConfiguration derives the write endpoint's configuration from the serving one.
//
// Derived rather than supplied, and that is the point of doing this in one process: authentication,
// the token service, the storage path and the read-only flag a garbage collection sets cannot drift
// between the two halves, because there is only one of each. What differs is exactly what has to:
//
//   - No proxying, so a push is answered by the store. `skipmodecleanup` survives, or this half would
//     decide the store was last written in another mode and delete it — measured, twelve gigabytes.
//   - Its own address, since two listeners in one process cannot share a port.
//   - No debug listener and no Prometheus instrumentation. The metrics namespace is registered
//     globally with prometheus, and registering the same collectors from a second app panics the
//     process on startup. The serving half's metrics cover the storage they share.
//   - Real client addresses from the ingress that fronts this listener, which is also the only half
//     that has one.
func writeEndpointConfiguration(config *configuration.Configuration) *configuration.Configuration {
	// A copy, so that nothing here is visible to the half that serves reads. The storage parameters
	// are copied a level deeper because that is a map, and a shallow copy would share it.
	writeConfig := *config

	writeConfig.Storage = configuration.Storage{}
	for kind, parameters := range config.Storage {
		copied := configuration.Parameters{}
		for key, value := range parameters {
			copied[key] = value
		}
		writeConfig.Storage[kind] = copied
	}

	writeConfig.Proxy = configuration.Proxy{SkipModeCleanup: true}
	writeConfig.WriteEndpoint = configuration.WriteEndpoint{}

	writeConfig.HTTP.Addr = config.WriteEndpoint.Addr
	writeConfig.HTTP.Debug.Addr = ""
	writeConfig.HTTP.Debug.Prometheus.Enabled = false

	writeConfig.HTTP.RealIP.Enabled = true
	writeConfig.HTTP.RealIP.ClientCert.CA = config.WriteEndpoint.ClientCertCA

	return &writeConfig
}
