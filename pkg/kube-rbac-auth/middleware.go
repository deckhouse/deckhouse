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
// sidecar, but inside the application: a client presents a bearer token, the token is checked with a
// TokenReview and access to a configured resource is checked with a SubjectAccessReview.
package kuberbacauth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
	authnclientv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	authzclientv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// Client is the part of kubernetes.Interface the middleware uses.
type Client interface {
	AuthenticationV1() authnclientv1.AuthenticationV1Interface
	AuthorizationV1() authzclientv1.AuthorizationV1Interface
}

// Middleware checks requests before they reach the wrapped handler.
type Middleware struct {
	authn        *authenticator
	authz        *authorizer
	rules        []Rule
	excludePaths []string
	timeout      time.Duration
	log          *slog.Logger
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
	return &Middleware{
		authn: &authenticator{
			client:  client.AuthenticationV1(),
			cache:   newDecisionCache(c.CacheSize, c.StaleCacheTTL, clock),
			okTTL:   c.TokenCacheTTL,
			failTTL: c.DenyCacheTTL,
			backoff: *c.RetryBackoff,
			log:     c.Logger,
		},
		authz: &authorizer{
			client:   client.AuthorizationV1(),
			cache:    newDecisionCache(c.CacheSize, c.StaleCacheTTL, clock),
			allowTTL: c.AllowCacheTTL,
			denyTTL:  c.DenyCacheTTL,
			backoff:  *c.RetryBackoff,
			log:      c.Logger,
		},
		rules:        c.Rules,
		excludePaths: c.ExcludePaths,
		timeout:      c.ReviewTimeout,
		log:          c.Logger,
	}, nil
}

// Handler wraps next. Requests to excluded paths go through as they are, requests that match a rule
// need a token with access to the rule's resource, and other requests get 404, as with kube-rbac-proxy.
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
		token := bearerToken(r)
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// One budget for both reviews: when the API server hangs, a cached decision is still
		// served before the scraper gives up.
		ctx, cancel := context.WithTimeout(r.Context(), m.timeout)
		defer cancel()

		user, err := m.authn.authenticate(ctx, token)
		if err != nil {
			m.log.Error("cannot check the token", slog.String("path", r.URL.Path), slog.String("error", err.Error()))
			http.Error(w, "Cannot check the token", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		verb := verbFor(r.Method)
		allowed, reason, err := m.authz.authorize(ctx, user, verb, attrs)
		if err != nil {
			m.log.Error("cannot check access", slog.String("user", user.Name), slog.String("path", r.URL.Path), slog.String("error", err.Error()))
			http.Error(w, "Cannot check access", http.StatusInternalServerError)
			return
		}
		if !allowed {
			m.log.Debug("access denied", slog.String("user", user.Name), slog.String("verb", verb),
				slog.String("resource", attrs.Resource+"/"+attrs.Subresource), slog.String("reason", reason))
			http.Error(w, fmt.Sprintf("Forbidden (user=%s, verb=%s, resource=%s, subresource=%s)",
				user.Name, verb, attrs.Resource, attrs.Subresource), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
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
