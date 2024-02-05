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
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/coreos/pkg/capnslog"
	"github.com/felixge/httpsnoop"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"basic-auth-proxy/pkg/proxy/provider"
)

const (
	certFilename = "client.crt"
	keyFilename  = "client.key"

	caFilepath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

var logger = capnslog.NewPackageLogger("basic-auth-proxy", "proxy")

var defaultFlushInterval = 50 * time.Millisecond

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func tlsHTTPClientTransport(certPath string) *http.Transport {
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(certPath, certFilename),
		filepath.Join(certPath, keyFilename),
	)
	if err != nil {
		logger.Fatalf("loading certificates: %+v", err)
	}

	caCerts := x509.NewCertPool()
	caCert, err := os.ReadFile(caFilepath)
	if err != nil {
		logger.Fatalf("append CA cert: %+v", err)
	}

	ok := caCerts.AppendCertsFromPEM(caCert)
	if !ok {
		logger.Fatal("failed to parse CA certificate")
	}

	transport := &http.Transport{
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
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCerts,
		},
	}
	return transport
}

type Handler struct {
	ListenAddress          string
	KubernetesAPIServerURL string
	CertPath               string

	CrowdBaseURL             string
	CrowdApplicationLogin    string
	CrowdApplicationPassword string
	CrowdGroups              []string

	OIDCBaseURL             string
	OIDCApplicationLogin    string
	OIDCApplicationPassword string
	OIDCGroups              []string
	OIDCScopes              []string

	AuthCacheTTL   time.Duration
	GroupsCacheTTL time.Duration

	Cache        *ttlcache.Cache
	reverseProxy *httputil.ReverseProxy

	provider provider.Provider

	PrometheusRegistry *prometheus.Registry
}

var _ http.Handler = &Handler{}

func NewHandler() *Handler {
	c := ttlcache.NewCache()
	c.SkipTtlExtensionOnHit(true)
	return &Handler{Cache: c, CrowdGroups: []string{}}
}

func (h *Handler) Run() {
	logger.Printf("-- Listening on: %s", h.ListenAddress)
	logger.Printf("-- Kubernetes API URL: %s", h.KubernetesAPIServerURL)
	logger.Printf("-- Auth Cache TTL: %v", h.AuthCacheTTL)
	logger.Printf("-- Groups Cache TTL: %v", h.GroupsCacheTTL)

	if h.CrowdBaseURL != "" && h.OIDCBaseURL != "" {
		logger.Fatal("only one auth provider can be used")
	}

	if h.CrowdBaseURL != "" {
		h.provider = provider.NewCrowdProvider(h.CrowdBaseURL, h.CrowdApplicationLogin, h.CrowdApplicationPassword, h.CrowdGroups)
		logger.Printf("-- Crowd URL: %s", h.CrowdBaseURL)
	}

	if h.OIDCBaseURL != "" {
		h.provider = provider.NewOIDCProvider(h.OIDCBaseURL, h.OIDCApplicationLogin, h.OIDCApplicationPassword, h.OIDCScopes, h.OIDCGroups)
		logger.Printf("-- OIDC URL: %s", h.OIDCBaseURL)
	}

	u, _ := url.Parse(h.KubernetesAPIServerURL)

	h.reverseProxy = httputil.NewSingleHostReverseProxy(u)
	h.reverseProxy.Transport = tlsHTTPClientTransport(h.CertPath)
	h.reverseProxy.FlushInterval = defaultFlushInterval

	h.PrometheusRegistry = prometheus.NewRegistry()
	requestCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests.",
	}, []string{"handler", "code", "method"})

	err := h.PrometheusRegistry.Register(requestCounter)
	if err != nil {
		logger.Fatalf("cannot register prometheus metrics: %s", err)
	}

	err = h.PrometheusRegistry.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	if err != nil {
		logger.Fatalf("cannot register process metrics: %s", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(h, w, r)
		requestCounter.With(prometheus.Labels{
			"handler": "/",
			"code":    strconv.Itoa(m.Code),
			"method":  r.Method,
		}).Inc()
	})
	http.Handle("/metrics", promhttp.HandlerFor(h.PrometheusRegistry, promhttp.HandlerOpts{}))
	http.HandleFunc("/healthz", healthz)
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		logger.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		_, err := client.Get(h.KubernetesAPIServerURL + "/version")
		if err != nil {
			logger.Error(err)
			http.Error(w, "Error", http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})

	logger.Fatal(http.ListenAndServe(h.ListenAddress, nil))
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	basicLogin, basicPassword, ok := r.BasicAuth()
	if !ok {
		logger.Error("401 Unauthorized, no basic auth credentials have been sent")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groups := h.validateCredentials(basicLogin, basicPassword)
	if len(groups) == 0 {
		logger.Errorf("403 Forbidden, Crowd authentication problem: User %s has no allowed groups", basicLogin)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	h.modifyRequest(w, r, basicLogin, groups)
}

func (h *Handler) validateCredentials(login, password string) []string {
	userID := login + ":" + password

	value, exists := h.Cache.Get(userID)
	if exists {
		if value != nil {
			return value.([]string)
		}
		return []string{}
	}

	groups, err := h.provider.ValidateCredentials(login, password)
	if err != nil {
		logger.Errorf("validating user credentials: %+v", err)
		h.Cache.SetWithTTL(userID, nil, h.AuthCacheTTL)
		return nil
	}

	h.Cache.SetWithTTL(userID, groups, h.GroupsCacheTTL)
	logger.Printf("received groups for %s: %s", login, groups)
	return groups
}

func (h *Handler) modifyRequest(w http.ResponseWriter, r *http.Request, login string, groups []string) {
	r.Header.Del("Authorization")
	r.Header.Set("X-Remote-User", login)

	for _, group := range groups {
		r.Header.Add("X-Remote-Group", group)
	}

	logger.Printf("%s [%s] %s --  %v", r.Method, r.Host, r.RequestURI, r.Header)
	h.reverseProxy.ServeHTTP(w, r)
}
