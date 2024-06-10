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
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"basic-auth-proxy/pkg/proxy/provider"
)

const (
	certFilename = "client.crt"
	keyFilename  = "client.key"

	caFilepath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	defaultFlushInterval = 50 * time.Second
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

	AuthCacheTTL   time.Duration
	GroupsCacheTTL time.Duration

	cache        *ttlcache.Cache
	reverseProxy *httputil.ReverseProxy

	logger *capnslog.PackageLogger

	provider provider.Provider

	prometheusRegistry *prometheus.Registry
}

func New() *Handler {
	c := ttlcache.NewCache()
	c.SkipTtlExtensionOnHit(true)
	return &Handler{
		cache:       c,
		CrowdGroups: []string{},
		OIDCScopes:  []string{},
		logger:      capnslog.NewPackageLogger("basic-auth-proxy", "proxy")}
}

func (h *Handler) Run() {
	h.logger.Printf("-- Listening on: %s", h.ListenAddress)
	h.logger.Printf("-- Kubernetes API URL: %s", h.KubernetesAPIServerURL)
	h.logger.Printf("-- Auth Cache TTL: %v", h.AuthCacheTTL)
	h.logger.Printf("-- Groups Cache TTL: %v", h.GroupsCacheTTL)

	if h.CrowdBaseURL != "" && h.OIDCBaseURL != "" {
		h.logger.Fatal("only one auth provider can be used")
	}

	if h.CrowdBaseURL != "" {
		h.provider = provider.NewCrowd(h.CrowdBaseURL, h.CrowdApplicationLogin, h.CrowdApplicationPassword, h.CrowdGroups)
		h.logger.Printf("-- Crowd URL: %s", h.CrowdBaseURL)
	}

	if h.OIDCBaseURL != "" {
		h.provider = provider.NewOIDC(h.OIDCBaseURL, h.OIDCClientID, h.OIDCClientSecret, h.OIDCGetUserInfo,
			h.OIDCBasicAuthUnsupported, h.OIDCScopes)
		h.logger.Printf("-- OIDC URL: %s", h.OIDCBaseURL)
	}

	u, _ := url.Parse(h.KubernetesAPIServerURL)

	h.reverseProxy = httputil.NewSingleHostReverseProxy(u)
	h.reverseProxy.Transport = h.buildHTTPTransport(h.CertPath)
	h.reverseProxy.FlushInterval = defaultFlushInterval

	h.prometheusRegistry = prometheus.NewRegistry()
	requestCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests.",
	}, []string{"handler", "code", "method"})

	err := h.prometheusRegistry.Register(requestCounter)
	if err != nil {
		h.logger.Fatalf("cannot register prometheus metrics: %s", err)
	}

	err = h.prometheusRegistry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if err != nil {
		h.logger.Fatalf("cannot register process metrics: %s", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(h, w, r)
		requestCounter.With(prometheus.Labels{
			"handler": "/",
			"code":    strconv.Itoa(m.Code),
			"method":  r.Method,
		}).Inc()
	})
	http.Handle("/metrics", promhttp.HandlerFor(h.prometheusRegistry, promhttp.HandlerOpts{}))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		h.logger.Fatal(err)
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
		if _, err = client.Get(h.KubernetesAPIServerURL + "/version"); err != nil {
			h.logger.Error(err)
			http.Error(w, "Error", http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})

	h.logger.Fatal(http.ListenAndServe(h.ListenAddress, nil))
}
func (h *Handler) buildHTTPTransport(certPath string) *http.Transport {
	cert, err := tls.LoadX509KeyPair(
		filepath.Join(certPath, certFilename),
		filepath.Join(certPath, keyFilename),
	)
	if err != nil {
		h.logger.Fatalf("loading certificates: %+v", err)
	}

	caCerts := x509.NewCertPool()
	caCert, err := os.ReadFile(caFilepath)
	if err != nil {
		h.logger.Fatalf("append CA cert: %+v", err)
	}

	ok := caCerts.AppendCertsFromPEM(caCert)
	if !ok {
		h.logger.Fatal("failed to parse CA certificate")
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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("receive a new request from ", r.RemoteAddr)
	basicLogin, basicPassword, ok := r.BasicAuth()
	if !ok {
		h.logger.Error("401 Unauthorized, no basic auth credentials have been sent")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groups, err := h.validateCredentials(basicLogin, basicPassword)
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

func (h *Handler) validateCredentials(login, password string) ([]string, error) {
	userID := login + ":" + password

	if value, exists := h.cache.Get(userID); exists {
		if value != nil {
			return value.([]string), nil
		}
		return []string{}, nil
	}

	groups, err := h.provider.ValidateCredentials(login, password)
	if err != nil {
		h.logger.Errorf("error during validating user credentials: %+v", err)
		h.cache.SetWithTTL(userID, nil, h.AuthCacheTTL)
		return nil, err
	}

	h.cache.SetWithTTL(userID, groups, h.GroupsCacheTTL)
	h.logger.Printf("received groups for %s: %s", login, groups)
	return groups, nil
}

func (h *Handler) modifyRequest(w http.ResponseWriter, r *http.Request, login string, groups []string) {
	r.Header.Del("Authorization")
	r.Header.Set("X-Remote-User", login)

	for _, group := range groups {
		r.Header.Add("X-Remote-Group", group)
	}

	h.logger.Printf("%s [%s] %s --  %v", r.Method, r.Host, r.RequestURI, r.Header)
	h.reverseProxy.ServeHTTP(w, r)
}
