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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-agent/internal/metrics"
)

const (
	// DefaultResponseTimeout is how long a target has to produce response headers.
	//
	// Short, because everything it covers is small: a connection, a challenge, a token,
	// and a set of headers. A registry that cannot manage those in a minute is one the
	// next target should be tried instead of.
	DefaultResponseTimeout = time.Minute

	// DefaultIdleTimeout is how long a transfer may produce nothing before it is
	// abandoned.
	//
	// Below containerd's `image_pull_progress_timeout`, which Deckhouse sets to five
	// minutes, so that a stalled transfer is the agent's to fail over rather than
	// containerd's to give up on. Generous all the same: it is not measuring how long
	// an image takes, only how long nothing at all has happened.
	DefaultIdleTimeout = 2 * time.Minute
)

// Layout provides the current node layout. An interface so the server does not care
// whether it came from the API server or from the copy on disk.
type Layout interface {
	Current() *registryv1alpha1.RegistryNodeSpec
}

// LayoutFunc adapts a function to Layout.
type LayoutFunc func() *registryv1alpha1.RegistryNodeSpec

func (f LayoutFunc) Current() *registryv1alpha1.RegistryNodeSpec { return f() }

// Server answers the container runtime's registry requests.
type Server struct {
	Log *slog.Logger

	// Layout is where the routing rules come from.
	Layout Layout

	// Self is the agent's own endpoint, used to refuse a request that names it.
	Self string

	// Metrics is what the agent reports about the pulls passing through it. Optional:
	// nothing about serving a pull may depend on whether anyone is watching.
	Metrics *metrics.Metrics

	// ResponseTimeout bounds getting an answer out of a target: the connection, the
	// authentication it demands, and the response headers. Without it a hung registry
	// would hold the pull for as long as it likes, and a fallback that never gets
	// tried is not a fallback.
	//
	// It deliberately does NOT bound the transfer that follows. A single deadline over
	// the whole attempt cannot tell a hung registry from a large image on a slow link:
	// a container disk of a virtual machine is tens of gigabytes, and any deadline
	// generous enough to carry it is far too generous to detect a registry that stopped
	// answering. What separates the two is not how long the transfer takes but whether
	// bytes are still arriving, which is what IdleTimeout measures.
	ResponseTimeout time.Duration

	// IdleTimeout is how long a transfer already under way may produce nothing before
	// it is abandoned.
	//
	// Reset by every byte that arrives, so a transfer that is merely slow runs for as
	// long as it needs, while one that has stalled is cut promptly. This is the same
	// shape as containerd's own `image_pull_progress_timeout`, and it has to stay below
	// it: whichever side gives up first decides what happens next, and the agent giving
	// up means the next target is tried, while containerd giving up means the pull
	// fails.
	IdleTimeout time.Duration

	// TrustDir is where Deckhouse stages the certificate authorities this cluster
	// accepts, one file per registry — see DefaultTrustDir. Empty leaves the agent with
	// its image's own bundle, which knows nothing the cluster was told.
	TrustDir string

	// trust is the staged authorities as last read, refreshed by RefreshTrust.
	trust atomic.Pointer[trustBundle]

	// clients are HTTP clients per certificate authority. Cached because building one
	// parses a certificate bundle, and that would otherwise happen on every layer of
	// every image. Replaced as a whole when the staged authorities change, so a cached
	// client can never outlive the trust it was built from.
	clients atomic.Pointer[clientCache]

	auth authenticator

	// serving reports whether the listener is up, for the status the agent publishes.
	serving atomic.Bool

	// failing records targets whose last attempt did not work, keyed by name.
	//
	// Kept as "known to be failing" rather than "known to work" on purpose: a node
	// that has pulled nothing yet has tried nothing, and reporting an empty set of
	// usable backends there would read as a broken agent rather than an idle one.
	failing sync.Map
}

// Serving reports whether the proxy is answering.
func (s *Server) Serving() bool { return s.serving.Load() }

// Usable narrows a list of backend names to those not known to be failing.
//
// Answers "which of these would I currently use", which is weaker than a live probe
// and deliberately so: probing every backend on every pass would put load on the
// registry proportional to the size of the cluster, for information that the next
// real pull produces for free.
func (s *Server) Usable(names []string) []string {
	usable := make([]string, 0, len(names))
	for _, name := range names {
		if _, broken := s.failing.Load(name); !broken {
			usable = append(usable, name)
		}
	}
	return usable
}

func (s *Server) markFailing(name string) { s.failing.Store(name, struct{}{}) }
func (s *Server) markWorking(name string) { s.failing.Delete(name) }

// Handler is the HTTP handler of the proxy.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// The runtime checks the endpoint before using it. Answered locally: it says
	// something about the agent, not about any registry, and making it depend on a
	// registry being reachable would take the node down with the registry.
	mux.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/v2/" {
			writer.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			writer.WriteHeader(http.StatusOK)
			return
		}
		s.forward(writer, request)
	})

	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	return mux
}

// Listen starts serving over TLS until the context is cancelled.
//
// The certificate comes from the configuration rather than from files named here, so it
// can be reloaded as it rotates: restarting the agent to pick up a new one would mean a
// window in which nothing on the node can pull.
func (s *Server) Listen(ctx context.Context, address string, tlsConfig *tls.Config) error {
	server := &http.Server{
		Addr:              address,
		Handler:           s.Handler(),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	s.serving.Store(true)
	defer s.serving.Store(false)

	// Empty paths: the certificate is served from TLSConfig.
	if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving on %s: %w", address, err)
	}
	return nil
}

// forward sends the request to the first target that answers.
func (s *Server) forward(writer http.ResponseWriter, request *http.Request) {
	namespace := request.URL.Query().Get("ns")

	decision, err := Resolve(namespace, request.URL.Path, s.Layout.Current(), s.Self)
	if err != nil {
		s.Log.Warn("cannot route the request",
			"namespace", namespace, "path", request.URL.Path, "error", err.Error())
		s.count("none", metrics.ResultUnroutable)
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}

	var lastStatus int
	var lastError error

	for i := range decision.Targets {
		target := &decision.Targets[i]

		response, err := s.attempt(request.Context(), request, target)
		if err != nil {
			lastError = err
			s.markFailing(target.Name)
			s.failover(target.Name, metrics.ReasonUnreachable)
			s.Log.Warn("a target did not answer, trying the next one",
				"target", target.Name, "host", target.Host, "error", err.Error())
			continue
		}

		// A target that refused credentials the agent supplied is a target that failed, and
		// its refusal must not be handed to the client.
		//
		// Which of the two it is turns on whose credentials were at stake, not on the status
		// code. Where the agent holds none it adds none and the client's own may still work,
		// so that challenge has to reach the client for a private pull to be possible at
		// all. Where the agent does hold them, they are the only ones that can work.
		//
		// Relaying that second kind is worse than useless: the challenge names the backend's
		// own token endpoint, so the client leaves the agent and dials a host it has no
		// reason to trust — reporting an unknown certificate authority for an address it
		// should never have contacted, while the next backend is never asked.
		refusedOurs := !target.Auth.IsEmpty() &&
			(response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden)
		if refusedOurs {
			lastStatus = response.StatusCode
			s.markFailing(target.Name)
			s.failover(target.Name, metrics.ReasonUnauthorized)
			_ = response.Body.Close()
			s.Log.Warn("a target refused the agent's credentials, trying the next one",
				"target", target.Name, "host", target.Host, "status", response.Status,
				"challenge", response.Header.Get("WWW-Authenticate"))
			continue
		}

		// Only a server-side failure is worth another target. A 404 means the image is
		// genuinely not there, and asking the upstream for it is exactly what the cache
		// is supposed to do — but that is the cache's job, not the agent's: retrying a
		// 404 here would turn every missing tag into a request to every backend.
		if response.StatusCode >= http.StatusInternalServerError && i+1 < len(decision.Targets) {
			lastStatus = response.StatusCode
			s.markFailing(target.Name)
			s.failover(target.Name, metrics.ReasonServerError)
			_ = response.Body.Close()
			s.Log.Warn("a target failed, trying the next one",
				"target", target.Name, "host", target.Host, "status", response.Status)
			continue
		}

		// It answered, so whatever was wrong with it before is no longer true.
		s.markWorking(target.Name)
		s.count(string(decision.Kind), result(response.StatusCode))
		s.relay(writer, response, request.URL.Path, target.Path)
		return
	}

	s.count(string(decision.Kind), metrics.ResultAllFailed)

	switch {
	case lastStatus != 0:
		http.Error(writer, fmt.Sprintf("every registry failed, the last with %d", lastStatus), http.StatusBadGateway)
	case lastError != nil:
		http.Error(writer, fmt.Sprintf("no registry could be reached: %v", lastError), http.StatusBadGateway)
	default:
		http.Error(writer, "no registry to try", http.StatusBadGateway)
	}
}

// result names what a registry's answer amounts to.
//
// A 404 is deliberately its own outcome and not a failure: a missing tag is the ordinary
// answer to a question about an image that does not exist, and counting it as an error
// would make every probe for an optional image look like a broken registry.
func result(status int) string {
	switch {
	case status == http.StatusNotFound:
		return metrics.ResultNotFound
	case status >= http.StatusInternalServerError:
		return metrics.ResultUnavailable
	default:
		return metrics.ResultServed
	}
}

func (s *Server) count(kind, outcome string) {
	if s.Metrics == nil {
		return
	}
	s.Metrics.Requests.WithLabelValues(kind, outcome).Inc()
}

func (s *Server) failover(target, reason string) {
	if s.Metrics == nil {
		return
	}
	s.Metrics.Failovers.WithLabelValues(target, reason).Inc()
}

// attempt sends the request to one target.
func (s *Server) attempt(
	ctx context.Context, original *http.Request, target *Target,
) (*http.Response, error) {
	client, err := s.client(target.CA)
	if err != nil {
		return nil, err
	}

	// Cancellable but without a deadline of its own. The phase that has to be bounded
	// in wall-clock time is answered by the transport's ResponseHeaderTimeout, and the
	// one that must not be is the body, so the only thing this context carries is the
	// ability to abandon the transfer — on a stalled read, or when the body is closed.
	ctx, cancel := context.WithCancel(ctx)

	request, err := http.NewRequestWithContext(ctx, original.Method, target.URL(), nil)
	if err != nil {
		cancel()
		return nil, err
	}

	// Only what the registry protocol needs. Copying the whole set would forward
	// cookies and anything else the client happens to carry to a third party.
	for _, header := range []string{"Accept", "Accept-Encoding", "Range", "User-Agent"} {
		for _, value := range original.Header.Values(header) {
			request.Header.Add(header, value)
		}
	}

	if target.Auth.IsEmpty() {
		// A target we hold no credentials for, reached with the client's own instead.
		//
		// Almost always a registry nobody configured. The credentials arrive here because
		// the runtime offers them to whatever host it contacts, and relaying them is the
		// only way a private third-party image can be pulled at all — the alternative is an
		// unconfigured registry that stops working the moment the agent is on the node.
		//
		// Keyed on whether WE have credentials rather than on the kind of decision: a
		// registry the cluster was given credentials for gets ours whichever rule routed
		// the request, one it was not gets the client's. The challenge travels back the same
		// way, since response headers are relayed as they are.
		for _, value := range original.Header.Values("Authorization") {
			request.Header.Add("Authorization", value)
		}
	} else {
		// A target the cluster holds credentials for is reached with those. The client's
		// own are deliberately NOT passed on here: they belong to whoever wrote the
		// imagePullSecret, and sending them to the Deckhouse upstream would hand them to a
		// party they were never meant for.
		//
		// Bounded on its own, and by wall-clock time rather than by progress: a challenge
		// and a token are small exchanges that belong to answering, not to transferring,
		// so they are held to the same deadline as the response headers they precede.
		authCtx, authCancel := context.WithTimeout(ctx, s.responseTimeout())
		err := s.auth.authorize(authCtx, client, request, target)
		authCancel()
		if err != nil {
			cancel()
			return nil, err
		}
	}

	response, err := client.Do(request)
	if err != nil {
		cancel()
		return nil, err
	}

	// The body is streamed to the client, so the request outlives this function and
	// the cancel has to travel with the body rather than fire here. Until it is closed,
	// what keeps a stalled transfer from lasting forever is the idle timeout, since the
	// context deliberately carries no deadline of its own.
	response.Body = newIdleBody(response.Body, s.idleTimeout(), cancel)
	return response, nil
}

// responseTimeout is how long a target has to answer, defaulted.
func (s *Server) responseTimeout() time.Duration {
	if s.ResponseTimeout > 0 {
		return s.ResponseTimeout
	}
	return DefaultResponseTimeout
}

// idleTimeout is how long a transfer may produce nothing, defaulted.
func (s *Server) idleTimeout() time.Duration {
	if s.IdleTimeout > 0 {
		return s.IdleTimeout
	}
	return DefaultIdleTimeout
}

// relay streams the response through, without buffering.
//
// Nothing is written to disk and nothing is held in memory: a layer can be gigabytes,
// and the agent runs on every node.
func (s *Server) relay(
	writer http.ResponseWriter, response *http.Response, requested, sent string,
) {
	defer func() { _ = response.Body.Close() }()

	for key, values := range response.Header {
		for _, value := range values {
			if http.CanonicalHeaderKey(key) == "Link" {
				value = restorePath(value, requested, sent)
			}
			writer.Header().Add(key, value)
		}
	}
	writer.WriteHeader(response.StatusCode)

	if _, err := io.Copy(writer, response.Body); err != nil {
		// The status line is already sent, so there is nothing to report to the client
		// except the shape of the answer itself: the runtime has to see a body that ends
		// where it should not, and retry from the offset it reached.
		s.Log.Warn("the transfer was interrupted", "error", err.Error())

		// Aborting the connection rather than returning, because returning would finish
		// the response cleanly. Where the upstream declared a Content-Length that is
		// merely untidy — the client counts the bytes and sees they are short — but a
		// chunked response has no count, so a truncated body would be terminated with a
		// correct final chunk and read as complete. The client would then verify a digest
		// over half an image and start again from nothing, with no indication of why.
		//
		// ErrAbortHandler is the standard library's way to say this: the server closes
		// the connection without logging a panic of its own.
		panic(http.ErrAbortHandler)
	}
}

// restorePath puts the path the client asked for back into a Link header.
//
// A registry paginates by handing out the URL of the next page, and the upstream builds that URL
// from ITS OWN repository name — which is not the one the client used. This proxy rewrites the path
// on the way out and has to undo that on the way back, or the client follows the header straight to
// a path this proxy does not serve.
//
// Left alone, the header names the upstream's path, the client follows it as given, and the prefix is
// applied a second time — `NAME_UNKNOWN` for a repository nobody asked about. Anything that pages
// through a listing hits this, module release lists among them, so a cluster stops converging.
//
// Only the path is touched, and only when it is the one that was sent: the query carries the
// upstream's own cursor, which is opaque and must survive untouched.
func restorePath(header, requested, sent string) string {
	if requested == "" || sent == "" || requested == sent {
		return header
	}
	return strings.ReplaceAll(header, sent, requested)
}

// clientCache holds the built clients. Its own type so the whole set can be swapped when
// the trust it was built from changes.
type clientCache struct {
	byAuthority sync.Map
}

// CloseIdle releases the pooled connections of every client in the cache, which is what
// makes replacing the cache free rather than a leak of open sockets.
func (c *clientCache) CloseIdle() {
	c.byAuthority.Range(func(_, value any) bool {
		value.(*http.Client).CloseIdleConnections()
		return true
	})
}

// client returns an HTTP client trusting the given certificate authority, together with
// the authorities Deckhouse staged on this node.
//
// The staged ones are added to every client, including the one for a target that named no
// authority of its own: that target is a registry the agent forwards to untouched, and
// under the agent it is the only party left that can verify it — every registry reaches
// the runtime through one drop-in, so the per-registry configuration the runtime used to
// hold is gone. Without them a ModuleSource with its own authority cannot be pulled from
// at all, while its certificate sits on the node's filesystem.
func (s *Server) client(certificateAuthority string) (*http.Client, error) {
	staged := s.trust.Load()

	key := certificateAuthority
	if staged != nil {
		key = staged.digest + "\x00" + certificateAuthority
	}

	cache := s.clients.Load()
	if cache == nil {
		cache = new(clientCache)
		if !s.clients.CompareAndSwap(nil, cache) {
			cache = s.clients.Load()
		}
	}

	if cached, ok := cache.byAuthority.Load(key); ok {
		return cached.(*http.Client), nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The one deadline that belongs on the transport rather than on a context: it stops
	// at the response headers and leaves the body alone, which is exactly the split this
	// proxy needs. A context deadline cannot express that — it would carry on into the
	// transfer and cut a large image that is arriving perfectly well.
	transport.ResponseHeaderTimeout = s.responseTimeout()
	if certificateAuthority != "" || (staged != nil && len(staged.pem) > 0) {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if staged != nil && len(staged.pem) > 0 {
			// Not an error when none of it parses: the directory is the node's, and one
			// unreadable file there must not take down the pull path. What a target
			// declared for itself, below, is a configuration this cluster wrote and is
			// held to a stricter standard.
			if !pool.AppendCertsFromPEM(staged.pem) {
				s.Log.Warn("none of the certificate authorities staged on the node could be parsed",
					"directory", s.TrustDir)
			}
		}
		if certificateAuthority != "" {
			if !pool.AppendCertsFromPEM([]byte(certificateAuthority)) {
				return nil, errors.New("the configured certificate authority is not valid PEM")
			}
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}

	client := &http.Client{
		Transport: transport,
		// Redirects are followed by hand nowhere: a registry redirecting a blob to
		// object storage is normal, and the default policy handles it. What must not
		// happen is carrying the Authorization header across hosts, which the standard
		// library already prevents.
	}

	actual, _ := cache.byAuthority.LoadOrStore(key, client)
	return actual.(*http.Client), nil
}

// idleBody ties a request's cancellation to the lifetime of its body, so a streamed
// response is not cut off by the function that started it returning, and abandons the
// transfer when it stops producing anything.
//
// Idleness rather than duration is what makes this safe to put on the path of an image
// of any size. A deadline would have to be set for the largest image on the slowest link
// the cluster will ever have, and at that length it no longer detects anything; a timer
// that every arriving byte pushes back stays short whatever the image weighs, because
// what it measures is silence.
type idleBody struct {
	io.ReadCloser

	idle   time.Duration
	cancel context.CancelFunc

	// mu guards the pair below. Without it the timer could fire between a read
	// returning bytes and that read rearming it, and the transfer would be abandoned
	// at the moment it demonstrated it was alive.
	mu      sync.Mutex
	timer   *time.Timer
	expired bool
}

func newIdleBody(body io.ReadCloser, idle time.Duration, cancel context.CancelFunc) io.ReadCloser {
	wrapped := &idleBody{ReadCloser: body, idle: idle, cancel: cancel}
	wrapped.timer = time.AfterFunc(idle, wrapped.expire)
	return wrapped
}

// expire abandons the request, which surfaces as a read error on the body.
func (b *idleBody) expire() {
	b.mu.Lock()
	b.expired = true
	b.mu.Unlock()
	b.cancel()
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.mu.Lock()
		if !b.expired {
			b.timer.Reset(b.idle)
		}
		b.mu.Unlock()
	}
	return n, err
}

func (b *idleBody) Close() error {
	b.mu.Lock()
	b.expired = true
	b.timer.Stop()
	b.mu.Unlock()

	err := b.ReadCloser.Close()
	b.cancel()
	return err
}
