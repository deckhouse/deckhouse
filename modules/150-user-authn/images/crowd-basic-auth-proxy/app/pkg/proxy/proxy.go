package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/coreos/pkg/capnslog"
)

const (
	certFilename = "client.crt"
	keyFilename  = "client.key"

	caFilepath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

var logger = capnslog.NewPackageLogger("crowd-auth-proxy", "proxy")

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
	caCert, err := ioutil.ReadFile(caFilepath)
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
	ListenAddress            string
	KubernetesAPIServerURL   string
	CertPath                 string
	CrowdBaseURL             string
	CrowdApplicationLogin    string
	CrowdApplicationPassword string
	CrowdGroups              []string
	CacheTTL                 time.Duration

	Cache        *ttlcache.Cache
	reverseProxy *httputil.ReverseProxy
	crowdClient  *CrowdClient
}

var _ http.Handler = &Handler{}

func NewHandler() *Handler {
	return &Handler{Cache: ttlcache.NewCache(), CrowdGroups: []string{}}
}

func (h *Handler) Run() {
	logger.Printf("-- Listening on: %s", h.ListenAddress)
	logger.Printf("-- Atlassian Crowd URL: %s", h.CrowdBaseURL)
	logger.Printf("-- Kubernetes API URL: %s", h.KubernetesAPIServerURL)
	logger.Printf("-- Cache TTL: %v", h.CacheTTL)

	u, _ := url.Parse(h.KubernetesAPIServerURL)

	h.reverseProxy = httputil.NewSingleHostReverseProxy(u)
	h.reverseProxy.Transport = tlsHTTPClientTransport(h.CertPath)
	h.reverseProxy.FlushInterval = defaultFlushInterval

	h.crowdClient = NewCrowdClient(h.CrowdBaseURL, h.CrowdApplicationLogin, h.CrowdApplicationPassword, h.CrowdGroups)

	http.Handle("/", h)
	http.HandleFunc("/healthz", healthz)
	logger.Fatal(http.ListenAndServe(h.ListenAddress, nil))
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	basicLogin, basicPassword, ok := r.BasicAuth()
	if !ok {
		logger.Error("401 Unauthorized, no basic auth credentials")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groups := h.validateCredentials(basicLogin, basicPassword)
	if len(groups) == 0 {
		logger.Error("403 Forbidden, Crowd authentication problem")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	logger.Printf("%s %v -- [%s] %s%s %v", basicLogin, groups, r.Method, r.Host, r.RequestURI, r.Header)
	h.modifyRequest(w, r, basicLogin, groups)
}

func (h *Handler) validateCredentials(login, password string) []string {
	value, exists := h.Cache.Get(login + ":" + password)
	if exists {
		if value != nil {
			return value.([]string)
		}
		return []string{}
	}

	_, err := h.crowdClient.MakeRequest("/session", "POST", struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: login, Password: password})
	if err != nil {
		logger.Errorf("validating user credentials: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}

	body, err := h.crowdClient.MakeRequest("/user/group/nested?username="+login, "GET", nil)
	if err != nil {
		logger.Errorf("getting user groups: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}

	crowdGroups, err := h.crowdClient.GetGroups(body)
	if err != nil {
		logger.Errorf("getting user groups: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}

	h.Cache.SetWithTTL(login+":"+password, crowdGroups, h.CacheTTL)
	logger.Printf("received groups for %s: %s", login, crowdGroups)
	return crowdGroups
}

func (h *Handler) modifyRequest(w http.ResponseWriter, r *http.Request, login string, groups []string) {
	r.Header.Del("Authorization")
	r.Header.Set("X-Remote-User", login)

	for _, group := range groups {
		r.Header.Add("X-Remote-Group", group)
	}

	h.reverseProxy.ServeHTTP(w, r)
}
