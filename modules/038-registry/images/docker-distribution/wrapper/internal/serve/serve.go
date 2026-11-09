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
// The serving one is the upstream project's own `registry.NewRegistry`, with TLS, tracing and the
// graceful drain as upstream wrote them. The debug and metrics listener is not among them — upstream
// opens it from its `serve` command, which is not run here — and is opened beside it: see
// debugListener. What this module adds
// hangs off upstream's own `registry.RegisterHandler`: the token proxy, and the decision about whose
// word on a client's address is taken.
//
// The second is the write endpoint, and it exists because the first is a pull-through cache —
// upstream's proxy stores answer every write with UNSUPPORTED, while this store is filled BY writes
// for as long as it has an upstream to serve from. So a second app over the same directory, from the
// same configuration with the proxy section removed, serves the pushes; hand-built rather than a
// second `registry.NewRegistry`, because health checks live in a process-global registry and a
// second registration panics. Where it runs, it is the only listener that writes: the first refuses
// a client's writes even with no upstream, when it is the store itself — see servingConfiguration.
//
// The third is the loopback rewriter, which the cache treats as its upstream. Nothing outside this
// process can reach it.
//
// One thing this package must do that a library cannot: import the providers the configuration
// names. Upstream looks those up by STRING in a registry each provider fills from its own `init`, so
// a process importing none reads a valid configuration and panics before any listener opens — and
// the tests cannot see it when the imports live in _test.go files, because then every test builds a
// binary the image does not have. The two the module needs are in the import block above with no
// comment beside them, because goimports rejects one there: `registry/auth/token` and
// `storage/driver/filesystem`.
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
	_ "github.com/distribution/distribution/v3/registry/auth/token"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	gorhandlers "github.com/gorilla/handlers"

	"github.com/deckhouse/registry-distribution/internal/authproxy"
	"github.com/deckhouse/registry-distribution/internal/bound"
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

// The clients the write endpoint admits, by the common name of the leaf they present.
//
// Both certificates come from this module's own authority, which is what the listener verifies them
// against first — see writeTLS. The names are checked on top of that because the authority alone
// admits more than these two: it signs every leaf for client authentication as well as server
// authentication (the signer's default profile), so the token service's certificate and any leaf a
// later change adds would otherwise be write clients too, by accident rather than by decision.
//
// Constants rather than configuration. They are the names hooks/v2/pki/pki.go issues the
// certificates under (CommonNameIngressClient and CommonNameDistribution), not something an
// operator tunes, and a list in a rendered file is one more thing that can disagree with the PKI
// it describes. Renaming either certificate there means changing it here in the same change.
const (
	// IngressClientName is the ingress controller's certificate, presented when it proxies a push
	// from the publication endpoint. The one client whose word about somebody else's address is
	// taken: see realip.
	IngressClientName = "registry-storage-ingress-client"

	// SyncerClientName is the storage's own serving certificate, which the syncer in the same pod
	// presents when it writes to this replica's store: the fill, the replication and the garbage
	// collection. Reused rather than a leaf of its own, because adding one to the generated state
	// makes the hook regenerate all of it, the authority every node trusts included.
	SyncerClientName = "registry-storage"
)

// measureEvery is how often the store is walked to replace the guard's running count with what is
// on disk. The count errs only toward refusing early between walks, so this is about how soon space
// a collection freed becomes usable again, not about safety.
const measureEvery = 30 * time.Second

// Run starts everything and returns when the serving listener stops.
func Run(ctx context.Context, distribution *configuration.Configuration, wrapper *config.Wrapper, log *slog.Logger) error {
	if distribution.Proxy.RemoteURL != "" {
		// The rendered configuration must not carry one: which upstream a miss goes to, and under
		// which path, is decided here — see the upstream package — and a second answer in the file
		// would be a second opinion nobody reconciles.
		return errors.New("the rendered configuration carries a proxy section; the upstream is " +
			"configured in this module's own file instead")
	}

	guard, err := storeGuard(distribution, wrapper, log)
	if err != nil {
		return err
	}
	go guard.Run(ctx, measureEvery)

	// The serving half's writes are the local copies of cache misses. A refusal in the middle of
	// one drops the copy and keeps the client's stream — see bound.Discard.
	distribution.Middleware = bounded(distribution.Middleware, guard, bound.Discard)

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

	// First, so that it sits right around distribution's own handler: a miss the store could not keep
	// must reach the client as the blob it is, and not the blob with an error after it. See settle.
	dregistry.RegisterHandler(func(_ *configuration.Configuration, handler http.Handler) http.Handler {
		return settle(handler)
	})

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
		wrapped, err := realip.Handler("", "", handler)
		if err != nil {
			log.Error("the client address handling could not be mounted", "error", err.Error())
			return handler
		}
		return wrapped
	})

	serving, err := dregistry.NewRegistry(ctx, servingConfiguration(distribution, wrapper))
	if err != nil {
		return fmt.Errorf("building the serving registry: %w", err)
	}

	// After NewRegistry, as upstream's `serve` does: the registry's metrics are registered by then.
	// A listener that cannot open is reported inside and does not stop the registry.
	_, _ = debugListener(distribution, log)

	if wrapper.WriteEndpoint.Address != "" {
		writing, listener, err := writeEndpoint(ctx, distribution, wrapper, guard, log)
		if err != nil {
			return err
		}
		go func() {
			if err := writing.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				// Fatal for the pushes, not for the pulls. Reported loudly and left to the
				// kubelet: a store that cannot be written is still a store the cluster pulls from,
				// and taking the serving half down with it would turn a failed push into an outage.
				log.Error("the write endpoint stopped", "error", err.Error())
			}
		}()
	}

	// A reissued certificate is served by starting again: see ErrCertificateChanged.
	reissued := make(chan struct{})
	if distribution.HTTP.TLS.Certificate != "" {
		self, _, _ := net.SplitHostPort(distribution.HTTP.Addr)
		go watchCertificate(ctx, distribution.HTTP.TLS.Certificate, distribution.HTTP.TLS.Key, self,
			certificateEvery, log, func() {
				close(reissued)
				shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
				defer cancel()
				if err := serving.Shutdown(shutdown); err != nil {
					log.Warn("the serving listener did not stop cleanly", "error", err.Error())
				}
			})
	}

	err = serving.ListenAndServe()
	select {
	case <-reissued:
		return ErrCertificateChanged
	default:
		return err
	}
}

// servingConfiguration is the configuration the serving listener runs on: the rendered one, made
// read-only for clients when this process also runs a write endpoint.
//
// With an upstream the serving half is a cache, and upstream's proxy refuses every write on its own.
// Without one it is the store itself, host-networked on the node address of every master with nothing
// in front of it, and it used to take a push or a delete from anybody who could reach the node and
// held the write or sync credentials — around the ingress, its `whitelistSourceRanges`, and the client
// certificate the write endpoint requires. Nothing in this module writes there: the syncer's fill,
// replication and garbage collection and `d8 mirror push` all go to the write endpoint. So where one
// exists, it is the only way to write to the store.
//
// Distribution's own read-only mode does it, and does exactly that: it drops the write routes from the
// app, so a refused write is answered 405 from inside it and appears in the access log, and it gates
// only what a client asks for, so a cache miss is still stored. The mode is read per app, so it lands
// on this half alone; the write endpoint derives its configuration from the one passed in, which keeps
// the flag as rendered: off.
//
// A process without a write endpoint runs on the rendered configuration as it is. That is the bundle's
// registry during a bootstrap and the static pod of the previous registry version, which share this
// binary, have no other listener, and are filled by pushes to this one.
func servingConfiguration(rendered *configuration.Configuration, wrapper *config.Wrapper) *configuration.Configuration {
	if wrapper.WriteEndpoint.Address == "" {
		return rendered
	}

	settings := *rendered
	settings.Storage = copyStorage(rendered.Storage)
	if settings.Storage["maintenance"] == nil {
		settings.Storage["maintenance"] = configuration.Parameters{}
	}
	// A new value rather than a key changed inside the rendered one: below the copied sections the
	// values are still the rendered configuration's, which the write endpoint derives from. The type
	// is the one distribution's parser produces and the only one its app accepts.
	settings.Storage["maintenance"]["readonly"] = map[any]any{"enabled": true}

	return &settings
}

// copyStorage copies the storage section down to each section's parameters.
//
// The parameters are maps, so a shallow copy would share them between the two halves; anything one
// half sets there has to be a new value under a key of its own copy, not a change inside a value.
func copyStorage(storage configuration.Storage) configuration.Storage {
	copied := configuration.Storage{}
	for kind, parameters := range storage {
		section := configuration.Parameters{}
		for key, value := range parameters {
			section[key] = value
		}
		copied[kind] = section
	}
	return copied
}

// writeEndpoint builds the listener that accepts a push, and the server that runs it on that listener.
func writeEndpoint(
	ctx context.Context, serving *configuration.Configuration, wrapper *config.Wrapper, guard *bound.Guard,
	log *slog.Logger,
) (*http.Server, net.Listener, error) {
	if wrapper.WriteEndpoint.Address == serving.HTTP.Addr {
		return nil, nil, fmt.Errorf(
			"the write endpoint and the serving endpoint are both on %s, and they are two "+
				"listeners in one process", serving.HTTP.Addr)
	}

	settings := writeConfiguration(serving, wrapper, guard)

	// Before the listener opens: a write endpoint that cannot require a client certificate must not
	// accept a connection at all, not even for as long as it takes to notice.
	var tlsConfig *tls.Config
	if settings.HTTP.TLS.Certificate != "" {
		var err error
		if tlsConfig, err = writeTLS(settings, wrapper); err != nil {
			return nil, nil, err
		}
	}

	app := handlers.NewApp(ctx, settings)
	// No RegisterHealthChecks: the health registry is global to the process and the serving half
	// has already registered the checks for this storage. A second registration panics, and there
	// is nothing it would watch that the first does not.

	handler, err := writeHandler(app, settings, wrapper, guard)
	if err != nil {
		return nil, nil, err
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}

	listener, err := net.Listen("tcp", wrapper.WriteEndpoint.Address)
	if err != nil {
		return nil, nil, fmt.Errorf("listening for the write endpoint on %s: %w",
			wrapper.WriteEndpoint.Address, err)
	}
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}

	log.Info("the write endpoint accepts pushes", "address", wrapper.WriteEndpoint.Address)

	return server, listener, nil
}

// writeHandler is everything the write endpoint puts in front of distribution's own app.
//
// Its own function so that the tests run exactly this chain, with a recorder in place of the app:
// which client's word about its address is taken is decided here, and a test of a copy of it would
// pass while this one changed.
func writeHandler(
	app http.Handler, settings *configuration.Configuration, wrapper *config.Wrapper, guard *bound.Guard,
) (http.Handler, error) {
	handler := app
	// Right in front of distribution, behind everything that decides who the client is: a full store
	// is answered with 507 and a reason, not with the unknown error the storage would produce.
	handler = bound.Handler(guard, handler)
	handler = health.Handler(handler)

	handler, err := authproxy.Handler(wrapper.AuthProxy, handler)
	if err != nil {
		return nil, err
	}
	// The one listener an ingress fronts, and therefore the one where a forwarded address can be
	// worth believing — from the ingress's certificate and no other. The syncer is admitted here
	// too, with a certificate from the same authority, and it fronts nobody.
	handler, err = realip.Handler(wrapper.WriteEndpoint.ClientCertCA, IngressClientName, handler)
	if err != nil {
		return nil, err
	}
	if !settings.Log.AccessLog.Disabled {
		handler = gorhandlers.CombinedLoggingHandler(os.Stdout, handler)
	}
	return handler, nil
}

// writeConfiguration derives the write endpoint's configuration from the serving one.
//
// Derived rather than rendered separately, and that is the point of running both in one process:
// authentication, the token service and the storage path cannot drift between the two halves,
// because there is only one of each. The serving half refuses a client's writes whenever this
// endpoint exists — see servingConfiguration — so the read-only flag passed in, and kept here, is the
// rendered one, which is off. What differs is exactly what has to: no proxying, so a push is answered
// by the store; its own address, since two listeners cannot share a port; no debug listener, because
// the metrics collectors are registered process-globally and a second registration panics; and the
// same guard in a different mode, because here the client sending the bytes is the one who should
// hear the refusal, and the sooner the better.
func writeConfiguration(
	serving *configuration.Configuration, wrapper *config.Wrapper, guard *bound.Guard,
) *configuration.Configuration {
	settings := *serving
	settings.Storage = copyStorage(serving.Storage)

	settings.Middleware = bounded(serving.Middleware, guard, bound.Fail)
	settings.Proxy = configuration.Proxy{}
	settings.HTTP.Addr = wrapper.WriteEndpoint.Address
	settings.HTTP.Debug.Addr = ""
	settings.HTTP.Debug.Prometheus.Enabled = false

	return &settings
}

// storeGuard is the one accounting of the store both halves write through.
//
// Measured before any listener opens: a guard that has not walked the store believes it empty, and
// would admit a budget's worth of writes on top of whatever is already there.
func storeGuard(distribution *configuration.Configuration, wrapper *config.Wrapper, log *slog.Logger) (*bound.Guard, error) {
	if kind := distribution.Storage.Type(); kind != "filesystem" {
		return nil, fmt.Errorf("the store is bounded on a filesystem, and the configuration names %q", kind)
	}
	root, _ := distribution.Storage.Parameters()["rootdirectory"].(string)
	if root == "" {
		return nil, errors.New("the configuration names no rootdirectory for the store")
	}

	guard := bound.New(root, wrapper.Store.Budget, wrapper.Store.ReserveFor).WithLogger(log)

	started := time.Now()
	if err := guard.Measure(); err != nil {
		return nil, fmt.Errorf("measuring the store before serving it: %w", err)
	}

	// Asked once so that a store starting full says so at startup — the guard logs the refusal
	// itself. Not a reason to stay down: a full store still serves everything it holds. It is also
	// the first measurement of the filesystem, which the reserve logged below is taken of.
	_ = guard.Refusal()

	log.Info("the store is bounded",
		"used", guard.Used(), "budget", wrapper.Store.Budget, "reserve", guard.Reserve(),
		"measured_in", time.Since(started).String())

	return guard, nil
}

// bounded returns the storage middleware list with the guard in front, in the given mode.
//
// A new list every time: the two halves are derived from one configuration, and a list shared
// between them would put the serving half's mode on the write endpoint or the other way round.
func bounded(middleware map[string][]configuration.Middleware, guard *bound.Guard, mode bound.Mode) map[string][]configuration.Middleware {
	result := make(map[string][]configuration.Middleware, len(middleware)+1)
	for kind, list := range middleware {
		result[kind] = append([]configuration.Middleware(nil), list...)
	}

	storage := make([]configuration.Middleware, 0, len(result["storage"])+1)
	for _, entry := range result["storage"] {
		if entry.Name != bound.MiddlewareName {
			storage = append(storage, entry)
		}
	}
	// Last in the list, which distribution applies outermost: whatever else wraps the driver, the
	// guard sees the write first.
	storage = append(storage, configuration.Middleware{
		Name:    bound.MiddlewareName,
		Options: configuration.Parameters(bound.Options(guard, mode)),
	})
	result["storage"] = storage

	return result
}

// writeTLS is the serving certificate plus the client certificate this listener requires.
//
// Required, not asked for. It used to be asked for only: a push without a certificate was served and
// merely did not get to say which address it came from, on the reasoning that admission was the
// token service's job. That left this listener — host-networked, on the node address of every master
// — taking pushes from anybody who could reach the node and held the write credentials, around the
// ingress and everything configured on it: `whitelistSourceRanges` applied to the publication
// endpoint and to nothing else. The Service in front of it was the same hole from inside the cluster.
//
// Now a handshake has to present a leaf of this module's authority, issued to one of the two clients
// that write here: the ingress controller proxying `d8 mirror push`, and the syncer beside this
// process filling its own store. Anything else is refused before a byte of HTTP is read, and the
// credentials are still checked after that; the certificate decides who may ask, the token service
// what they may do. See IngressClientName for why the names are checked as well as the authority.
//
// Without an authority there is nothing to require, and a write endpoint serving TLS to anybody is
// exactly what this replaced, so that is a refusal to start rather than a fallback. The syncer
// renders the authority on every configuration.
func writeTLS(settings *configuration.Configuration, wrapper *config.Wrapper) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(settings.HTTP.TLS.Certificate, settings.HTTP.TLS.Key)
	if err != nil {
		return nil, fmt.Errorf("loading the write endpoint's certificate: %w", err)
	}

	if wrapper.WriteEndpoint.ClientCertCA == "" {
		return nil, errors.New("the write endpoint serves TLS and names no client certificate " +
			"authority (writeEndpoint.clientCertCA), so it would take a push from anybody who can " +
			"reach the node")
	}
	pem, err := os.ReadFile(wrapper.WriteEndpoint.ClientCertCA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", wrapper.WriteEndpoint.ClientCertCA, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no certificate this can use", wrapper.WriteEndpoint.ClientCertCA)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
		// Verified against the pool for client authentication by the handshake itself, before
		// VerifyConnection is asked about the name.
		ClientAuth:       tls.RequireAndVerifyClientCert,
		ClientCAs:        pool,
		VerifyConnection: admitWriters,
	}, nil
}

// admitWriters refuses a verified client whose leaf is not issued to one of the write endpoint's
// clients.
//
// The leaf only: it is the certificate the peer proved it holds the key for. Whatever else it sent
// is a chain, read by the verification as intermediates and nothing more.
func admitWriters(state tls.ConnectionState) error {
	if len(state.PeerCertificates) == 0 {
		// RequireAndVerifyClientCert has refused this already; stated so that a change of the
		// client authentication mode cannot quietly turn this into an open door.
		return errors.New("the write endpoint requires a client certificate")
	}
	switch name := state.PeerCertificates[0].Subject.CommonName; name {
	case IngressClientName, SyncerClientName:
		return nil
	default:
		return fmt.Errorf("the write endpoint does not admit a client certificate issued to %q", name)
	}
}
