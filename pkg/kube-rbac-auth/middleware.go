// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kuberbacauth protects HTTP endpoints, such as /metrics, in the same way as the kube-rbac-proxy
// sidecar, but inside the application: a client presents a bearer token or a client certificate,
// the token is checked with a TokenReview, the certificate is checked against the configured CAs,
// and access to a configured resource is checked with a SubjectAccessReview.
package kuberbacauth

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
	authnclientv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authzclientv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/deckhouse/deckhouse/pkg/kube-rbac-auth/internal/metrics"
)

// Client is the part of kubernetes.Interface the middleware uses.
type Client interface {
	AuthenticationV1() authnclientv1.AuthenticationV1Interface
	AuthorizationV1() authzclientv1.AuthorizationV1Interface
}

// Middleware checks requests before they reach the wrapped handler.
type Middleware struct {
	authn         *authenticator
	authz         *authorizer
	rules         []Rule
	excludePaths  []string
	timeout       time.Duration
	log           Logger
	metricStorage metricsstorage.Storage

	// certs is nil when client certificates are turned off.
	certs *certAuthenticator
}

// New returns a Middleware. The application's service account needs to create
// tokenreviews.authentication.k8s.io and subjectaccessreviews.authorization.k8s.io.
func New(client Client, cfg Config) (*Middleware, error) {
	return newWithClock(client, cfg, realClock{})
}

func newWithClock(client Client, cfg Config, clock utilcache.Clock) (*Middleware, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	c := cfg.withDefaults()

	if err := metrics.RegisterKubeRBACAuthMetrics(c.MetricStorage); err != nil {
		return nil, fmt.Errorf("register metrics: %w", err)
	}

	var certs *certAuthenticator
	if c.ClientCAFile != "" {
		cas, err := newClientCAs(c.ClientCAFile, clock, c.Logger)
		if err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		certs = newCertAuthenticator(cas, c.CacheSize, clock, c.Logger)
	}

	return &Middleware{
		certs: certs,
		authn: &authenticator{
			client:        client.AuthenticationV1(),
			cache:         newDecisionCache(c.CacheSize, c.StaleCacheTTL, clock),
			okTTL:         c.TokenCacheTTL,
			failTTL:       c.DenyCacheTTL,
			backoff:       *c.RetryBackoff,
			log:           c.Logger,
			metricStorage: c.MetricStorage,
		},
		authz: &authorizer{
			client:        client.AuthorizationV1(),
			cache:         newDecisionCache(c.CacheSize, c.StaleCacheTTL, clock),
			allowTTL:      c.AllowCacheTTL,
			denyTTL:       c.DenyCacheTTL,
			backoff:       *c.RetryBackoff,
			log:           c.Logger,
			metricStorage: c.MetricStorage,
		},
		rules:         c.Rules,
		excludePaths:  c.ExcludePaths,
		timeout:       c.ReviewTimeout,
		log:           c.Logger,
		metricStorage: c.MetricStorage,
	}, nil
}

// Handler wraps next. Requests to excluded paths go through as they are, requests that match a rule
// need a token or a client certificate with access to the rule's resource, and other requests get 404,
// as with kube-rbac-proxy.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	for _, rule := range m.rules {
		mux.Handle(rule.Path, m.protect(rule.ResourceAttributes, next))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.excluded(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// ClientCAs returns a copy of the CAs that sign the accepted client certificates, or nil
// if client certificates are turned off. A server names them in the handshake, so a client
// with several certificates sends the right one: see ServerConfig.ClientCAs.
func (m *Middleware) ClientCAs() *x509.CertPool {
	if m.certs == nil {
		return nil
	}
	cas, _ := m.certs.cas.get()
	if cas == nil {
		return nil
	}
	return cas.Clone()
}

func (m *Middleware) excluded(p string) bool {
	for _, pattern := range m.excludePaths {
		if ok, _ := path.Match(pattern, p); ok {
			return true
		}
	}
	return false
}

func (m *Middleware) protect(attrs ResourceAttributes, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One budget for both reviews: when the API server hangs, a cached decision is still
		// served before the scraper gives up.
		ctx, cancel := context.WithTimeout(r.Context(), m.timeout)
		defer cancel()

		user, credential, err := m.authenticate(ctx, r)
		if err != nil {
			m.log.Error("cannot check the token", slog.String("path", r.URL.Path), slog.String("error", err.Error()))
			m.metricStorage.CounterAdd(metrics.RequestsTotal, 1, map[string]string{metrics.LabelResult: metrics.ResultError, metrics.LabelCredential: credential})
			http.Error(w, "Cannot check the token", http.StatusInternalServerError)
			return
		}
		if user == nil {
			m.metricStorage.CounterAdd(metrics.RequestsTotal, 1, map[string]string{metrics.LabelResult: metrics.ResultUnauthorized, metrics.LabelCredential: credential})
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		verb := verbFor(r.Method)
		allowed, reason, err := m.authz.authorize(ctx, user, verb, attrs)
		if err != nil {
			m.log.Error("cannot check access", slog.String("user", user.Name), slog.String("path", r.URL.Path), slog.String("error", err.Error()))
			m.metricStorage.CounterAdd(metrics.RequestsTotal, 1, map[string]string{metrics.LabelResult: metrics.ResultError, metrics.LabelCredential: credential})
			http.Error(w, "Cannot check access", http.StatusInternalServerError)
			return
		}
		if !allowed {
			m.log.Debug("access denied", slog.String("user", user.Name), slog.String("verb", verb),
				slog.String("resource", attrs.Resource+"/"+attrs.Subresource), slog.String("reason", reason))
			m.metricStorage.CounterAdd(metrics.RequestsTotal, 1, map[string]string{metrics.LabelResult: metrics.ResultForbidden, metrics.LabelCredential: credential})
			http.Error(w, fmt.Sprintf("Forbidden (user=%s, verb=%s, resource=%s, subresource=%s)",
				user.Name, verb, attrs.Resource, attrs.Subresource), http.StatusForbidden)
			return
		}

		m.metricStorage.CounterAdd(metrics.RequestsTotal, 1, map[string]string{metrics.LabelResult: metrics.ResultAllowed, metrics.LabelCredential: credential})
		next.ServeHTTP(w, r)
	})
}

// authenticate returns nil and no error when the request has no valid credentials, and the credential
// the answer is about, one of the metrics.Credential constants.
// A trusted client certificate wins over a token, as in the API server.
// A certificate from an unknown CA is ignored, so its owner can still use a token.
func (m *Middleware) authenticate(ctx context.Context, r *http.Request) (*User, string, error) {
	if m.certs != nil {
		if user := m.certs.authenticate(r); user != nil {
			return user, metrics.CredentialCertificate, nil
		}
	}
	token := bearerToken(r)
	if token == "" {
		return nil, metrics.CredentialNone, nil
	}
	user, err := m.authn.authenticate(ctx, token)
	return user, metrics.CredentialToken, err
}

// bearerToken accepts the scheme in any letter case, as RFC 7235 requires.
func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// verbFor maps an HTTP method to a Kubernetes verb as kube-rbac-proxy does. HEAD is a read here,
// and any other method needs the "*" verb, which only a wildcard role grants.
func verbFor(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return "get"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		return "delete"
	default:
		return "*"
	}
}
