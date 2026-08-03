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
	"sync"
	"sync/atomic"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-agent/internal/metrics"
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

	// ForwardTimeout bounds one attempt at one target. Without it a hung registry
	// would hold the pull for as long as it likes, and a fallback that never gets
	// tried is not a fallback.
	ForwardTimeout time.Duration

	// clients are HTTP clients per certificate authority. Cached because building one
	// parses a certificate bundle, and that would otherwise happen on every layer of
	// every image.
	clients sync.Map

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

	repository, _, err := splitAPIPath(request.URL.Path)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}

	var lastStatus int
	var lastError error

	for i := range decision.Targets {
		target := &decision.Targets[i]

		response, err := s.attempt(request.Context(), request, target, repository, decision.Kind)
		if err != nil {
			lastError = err
			s.markFailing(target.Name)
			s.failover(target.Name, metrics.ReasonUnreachable)
			s.Log.Warn("a target did not answer, trying the next one",
				"target", target.Name, "host", target.Host, "error", err.Error())
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
		s.relay(writer, response)
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
	ctx context.Context, original *http.Request, target *Target, repository string, kind Kind,
) (*http.Response, error) {
	client, err := s.client(target.CA)
	if err != nil {
		return nil, err
	}

	timeout := s.ForwardTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)

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

	if kind == KindPassThrough {
		// A registry nobody configured, reached with the pod's own imagePullSecret.
		//
		// The credentials arrive here because the runtime offers them to whatever host it
		// contacts: the kubelet leaves the server address unset, so containerd's host
		// check is skipped and the agent is offered them like any registry would be.
		// Relaying them is the only way a private third-party image can be pulled at all,
		// and the alternative — an unconfigured registry that stops working the moment
		// the agent is on the node — would break workloads that never asked to be
		// involved.
		//
		// The challenge travels back the same way, because response headers are relayed
		// as they are: the client sees the target's own 401 and answers it.
		for _, value := range original.Header.Values("Authorization") {
			request.Header.Add("Authorization", value)
		}
	} else if err := s.auth.authorize(ctx, client, request, target, repository); err != nil {
		// A configured target is reached with the credentials the cluster holds for it.
		// The client's own are deliberately NOT passed on here: they belong to whoever
		// wrote the imagePullSecret, and sending them to the Deckhouse upstream would
		// hand them to a party they were never meant for.
		cancel()
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		cancel()
		return nil, err
	}

	// The body is streamed to the client, so the request outlives this function and
	// the cancel has to travel with the body rather than fire here.
	response.Body = &cancelOnClose{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

// relay streams the response through, without buffering.
//
// Nothing is written to disk and nothing is held in memory: a layer can be gigabytes,
// and the agent runs on every node.
func (s *Server) relay(writer http.ResponseWriter, response *http.Response) {
	defer func() { _ = response.Body.Close() }()

	for key, values := range response.Header {
		for _, value := range values {
			writer.Header().Add(key, value)
		}
	}
	writer.WriteHeader(response.StatusCode)

	if _, err := io.Copy(writer, response.Body); err != nil {
		// The status line is already sent, so there is nothing to report to the client.
		// The runtime sees a truncated body and retries, which is the correct outcome.
		s.Log.Warn("the transfer was interrupted", "error", err.Error())
	}
}

// client returns an HTTP client trusting the given certificate authority.
func (s *Server) client(certificateAuthority string) (*http.Client, error) {
	if cached, ok := s.clients.Load(certificateAuthority); ok {
		return cached.(*http.Client), nil
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if certificateAuthority != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(certificateAuthority)) {
			return nil, errors.New("the configured certificate authority is not valid PEM")
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

	actual, _ := s.clients.LoadOrStore(certificateAuthority, client)
	return actual.(*http.Client), nil
}

// cancelOnClose ties a request's cancellation to the lifetime of its body, so a
// streamed response is not cut off by the function that started it returning.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
