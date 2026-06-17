/*
Copyright 2021 Flant JSC

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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/coreos/pkg/capnslog"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"basic-auth-proxy/pkg/proxy/provider"
)

const (
	certFilename = "client.crt"
	keyFilename  = "client.key"

	caFilepath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	defaultFlushInterval = 50 * time.Second

	// extraHeaderPrefix is the prefix kube-apiserver maps to user.Info.Extra
	// (see --requestheader-extra-headers-prefix); must be stripped on ingress.
	extraHeaderPrefix = "X-Remote-Extra-"
)

var _ http.Handler = &Handler{}

type Handler struct {
	ListenAddress          string
	KubernetesAPIServerURL string
	CertPath               string

	CrowdBaseURL             string
	CrowdApplicationLogin    string
	CrowdApplicationPassword string
	CrowdGroups              []string

	OIDCBaseURL              string
	OIDCClientID             string
	OIDCClientSecret         string
	OIDCScopes               []string
	OIDCBasicAuthUnsupported bool
	OIDCGetUserInfo          bool

	LDAPBaseURL              string
	LDAPClientID             string
	LDAPClientSecret         string
	LDAPScopes               []string
	LDAPBasicAuthUnsupported bool
	LDAPGetUserInfo          bool

	AuthCacheTTL   time.Duration
	GroupsCacheTTL time.Duration

	cache        *ttlcache.Cache
	reverseProxy *httputil.ReverseProxy

	logger *capnslog.PackageLogger

	provider provider.Provider

	prometheusRegistry *prometheus.Registry

	// cacheKeyHMACKey is a per-process random key for HMAC-SHA256 over
	// (login, password) so the raw password never lands in the cache map.
	cacheKeyHMACKey []byte
}

func New() *Handler {
	c := ttlcache.NewCache()
	c.SkipTtlExtensionOnHit(true)

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Errorf("init cache HMAC key: %w", err))
	}

	return &Handler{
		cache:           c,
		CrowdGroups:     []string{},
		OIDCScopes:      []string{},
		LDAPScopes:      []string{},
		logger:          capnslog.NewPackageLogger("basic-auth-proxy", "proxy"),
		cacheKeyHMACKey: key,
	}
}

// Run wires the proxy together and serves until the listener fails.
// Setup errors are returned so main can exit with a non-zero code.
func (h *Handler) Run(ctx context.Context) error {
	h.logger.Printf("-- Listening on: %s", h.ListenAddress)
	h.logger.Printf("-- Kubernetes API URL: %s", h.KubernetesAPIServerURL)
	h.logger.Printf("-- Auth Cache TTL: %v", h.AuthCacheTTL)
	h.logger.Printf("-- Groups Cache TTL: %v", h.GroupsCacheTTL)

	if err := h.initProvider(ctx); err != nil {
		return err
	}

	u, err := url.Parse(h.KubernetesAPIServerURL)
	if err != nil {
		return fmt.Errorf("parse api-server-url %q: %w", h.KubernetesAPIServerURL, err)
	}

	transport, err := h.buildHTTPTransport(h.CertPath)
	if err != nil {
		return err
	}

	h.reverseProxy = httputil.NewSingleHostReverseProxy(u)
	h.reverseProxy.Transport = transport
	h.reverseProxy.FlushInterval = defaultFlushInterval

	mux := http.NewServeMux()

	h.prometheusRegistry = prometheus.NewRegistry()
	requestCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests.",
	}, []string{"handler", "code", "method"})

	if err := h.prometheusRegistry.Register(requestCounter); err != nil {
		return fmt.Errorf("register prometheus metrics: %w", err)
	}
	if err := h.prometheusRegistry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return fmt.Errorf("register process metrics: %w", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(h, w, r)
		requestCounter.With(prometheus.Labels{
			"handler": "/",
			"code":    strconv.Itoa(m.Code),
			"method":  r.Method,
		}).Inc()
	})
	mux.Handle("/metrics", promhttp.HandlerFor(h.prometheusRegistry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	caCert, err := os.ReadFile(caFilepath)
	if err != nil {
		return fmt.Errorf("read api-server CA %q: %w", caFilepath, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("parse api-server CA %q", caFilepath)
	}

	readyClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    caCertPool,
			},
		},
	}

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, h.KubernetesAPIServerURL+"/version", nil)
		if reqErr != nil {
			h.logger.Error(reqErr)
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
		resp, getErr := readyClient.Do(req)
		if getErr != nil {
			h.logger.Error(getErr)
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
		_ = resp.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              h.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	serverErrCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	select {
	case err := <-serverErrCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}

func (h *Handler) initProvider(ctx context.Context) error {
	enabledProviders := 0
	if h.CrowdBaseURL != "" {
		enabledProviders++
	}
	if h.OIDCBaseURL != "" {
		enabledProviders++
	}
	if h.LDAPBaseURL != "" {
		enabledProviders++
	}
	if enabledProviders > 1 {
		return fmt.Errorf("only one auth provider can be configured, got %d", enabledProviders)
	}

	switch {
	case h.CrowdBaseURL != "":
		p, err := provider.NewCrowd(provider.CrowdConfig{
			APIURL:        h.CrowdBaseURL,
			Login:         h.CrowdApplicationLogin,
			Password:      h.CrowdApplicationPassword,
			AllowedGroups: h.CrowdGroups,
		})
		if err != nil {
			return fmt.Errorf("init Crowd provider: %w", err)
		}
		h.provider = p
		h.logger.Printf("-- Crowd URL: %s", h.CrowdBaseURL)

	case h.OIDCBaseURL != "":
		p, err := provider.NewOIDC(ctx, provider.OIDCConfig{
			URL:                  h.OIDCBaseURL,
			ClientID:             h.OIDCClientID,
			ClientSecret:         h.OIDCClientSecret,
			Scopes:               h.OIDCScopes,
			GetUserInfo:          h.OIDCGetUserInfo,
			BasicAuthUnsupported: h.OIDCBasicAuthUnsupported,
		})
		if err != nil {
			return fmt.Errorf("init OIDC provider: %w", err)
		}
		h.provider = p
		h.logger.Printf("-- OIDC URL: %s", h.OIDCBaseURL)

	case h.LDAPBaseURL != "":
		p, err := provider.NewLDAP(ctx, provider.LDAPConfig{
			URL:                  h.LDAPBaseURL,
			ClientID:             h.LDAPClientID,
			ClientSecret:         h.LDAPClientSecret,
			Scopes:               h.LDAPScopes,
			GetUserInfo:          h.LDAPGetUserInfo,
			BasicAuthUnsupported: h.LDAPBasicAuthUnsupported,
		})
		if err != nil {
			return fmt.Errorf("init LDAP provider: %w", err)
		}
		h.provider = p
		h.logger.Printf("-- LDAP OIDC URL: %s", h.LDAPBaseURL)

	default:
		return fmt.Errorf("no auth provider configured")
	}
	return nil
}

func (h *Handler) buildHTTPTransport(certPath string) (*http.Transport, error) {
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(certPath, certFilename),
		filepath.Join(certPath, keyFilename),
	)
	if err != nil {
		return nil, fmt.Errorf("loading client certificates from %q: %w", certPath, err)
	}

	caCerts := x509.NewCertPool()
	caCert, err := os.ReadFile(caFilepath)
	if err != nil {
		return nil, fmt.Errorf("read api-server CA %q: %w", caFilepath, err)
	}
	if !caCerts.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("parse api-server CA %q", caFilepath)
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCerts,
		},
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("receive a new request from ", r.RemoteAddr)
	basicLogin, basicPassword, ok := r.BasicAuth()
	if !ok {
		h.logger.Error("401 Unauthorized, no basic auth credentials have been sent")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groups, err := h.validateCredentials(r.Context(), basicLogin, basicPassword)
	if err != nil {
		h.logger.Errorf("403 Forbidden, authentication problem: %s", err.Error())
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.CrowdBaseURL != "" && len(groups) == 0 {
		h.logger.Errorf("403 Forbidden, Crowd authentication problem: User %s has no allowed groups", basicLogin)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.modifyRequest(w, r, basicLogin, groups)
}

// cacheEntry distinguishes a negative result (err != nil, AuthCacheTTL)
// from a successful auth with empty groups (err == nil, GroupsCacheTTL).
// Collapsing both into a bare nil caused the auth-bypass on cache hit.
type cacheEntry struct {
	groups []string
	err    error
}

// cacheKey returns HMAC-SHA256 over length-prefixed (login, password).
// Length-prefixing avoids collisions like ("ab","cd") vs ("a","bcd").
func (h *Handler) cacheKey(login, password string) string {
	mac := hmac.New(sha256.New, h.cacheKeyHMACKey)
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(login)))
	_, _ = mac.Write(lenBuf[:])
	_, _ = mac.Write([]byte(login))
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(password)))
	_, _ = mac.Write(lenBuf[:])
	_, _ = mac.Write([]byte(password))
	return string(mac.Sum(nil))
}

func (h *Handler) validateCredentials(ctx context.Context, login, password string) ([]string, error) {
	key := h.cacheKey(login, password)

	if value, exists := h.cache.Get(key); exists {
		entry, ok := value.(cacheEntry)
		if !ok {
			h.logger.Errorf("unexpected cache entry type %T, ignoring", value)
		} else {
			return entry.groups, entry.err
		}
	}

	groups, err := h.provider.ValidateCredentials(ctx, login, password)
	if err != nil {
		h.logger.Errorf("error during validating user credentials: %+v", err)
		h.cache.SetWithTTL(key, cacheEntry{err: err}, h.AuthCacheTTL)
		return nil, err
	}

	h.cache.SetWithTTL(key, cacheEntry{groups: groups}, h.GroupsCacheTTL)
	h.logger.Printf("received groups for %s: %s", login, groups)
	return groups, nil
}

// stripIdentityHeaders drops client-supplied X-Remote-* identity headers.
// kube-apiserver trusts them on our mTLS channel, so leaving them through
// would let any client become cluster-admin via `X-Remote-Group: system:masters`.
func stripIdentityHeaders(h http.Header) {
	h.Del("Authorization")
	h.Del("X-Remote-User")
	h.Del("X-Remote-Group")
	for k := range h {
		if strings.HasPrefix(k, extraHeaderPrefix) {
			h.Del(k)
		}
	}
}

func (h *Handler) modifyRequest(w http.ResponseWriter, r *http.Request, login string, groups []string) {
	stripIdentityHeaders(r.Header)
	r.Header.Set("X-Remote-User", login)
	for _, group := range groups {
		r.Header.Add("X-Remote-Group", group)
	}

	h.logger.Printf("%s [%s] %s --  %v", r.Method, r.Host, r.RequestURI, r.Header)
	h.reverseProxy.ServeHTTP(w, r)
}
