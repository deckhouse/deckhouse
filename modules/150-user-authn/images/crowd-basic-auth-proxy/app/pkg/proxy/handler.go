package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/ReneKroon/ttlcache"
)

var (
	certFilename = "client.crt"
	keyFilename  = "client.key"
	caDirpath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	_ http.Handler = &handler{}
)

type handler struct {
	CrowdBaseUrl           string
	KubernetesApiServerURL string
	ClientID               string
	ClientSecret           string
	HTTPClient             *http.Client
	CacheTTL               time.Duration
	Cache                  *ttlcache.Cache
}

func newHandler(crowdBaseUrl, apiServerUrl, login, password, certPath string, cacheTTL int) handler {
	cert, err := tls.LoadX509KeyPair(certPath+"/"+certFilename, certPath+"/"+keyFilename)
	if err != nil {
		logger.Fatalf("loading certificates: %+v", err)
	}
	caCerts := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(caDirpath)
	if err != nil {
		logger.Fatalf("append CA cert: %+v", err)
	}
	ok := caCerts.AppendCertsFromPEM(caCert)
	if !ok {
		logger.Fatal("failed to parse CA certificate")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: caCerts},
	}
	return handler{
		CrowdBaseUrl:           crowdBaseUrl,
		KubernetesApiServerURL: apiServerUrl,
		ClientID:               login,
		ClientSecret:           password,
		HTTPClient:             &http.Client{Transport: tr, Timeout: 60 * time.Second},
		CacheTTL:               time.Duration(cacheTTL) * time.Second,
		Cache:                  ttlcache.NewCache(),
	}
}

func (h *handler) validateCredentials(login, password string) []string {
	value, exists := h.Cache.Get(login + ":" + password)
	if exists {
		if value != nil {
			return value.([]string)
		}
		return []string{}
	}
	_, err := makeCrowdRequest(*h, "POST", "session", struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{Username: login, Password: password})
	if err != nil {
		logger.Errorf("validating user credentials: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}
	body, err := makeCrowdRequest(*h, "GET", "user/group/nested?username="+login, nil)
	if err != nil {
		logger.Errorf("getting user groups: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}

	crowdGroups, err := getCrowdGroups(body)
	if err != nil {
		logger.Errorf("getting user groups: %+v", err)
		h.Cache.SetWithTTL(login+":"+password, nil, h.CacheTTL)
		return nil
	}
	h.Cache.SetWithTTL(login+":"+password, crowdGroups, h.CacheTTL)
	logger.Printf("received groups for %s: %s", login, crowdGroups)
	return crowdGroups
}

func (h *handler) modifyRequest(w http.ResponseWriter, r *http.Request, login string, groups []string) {
	u, _ := url.Parse(h.KubernetesApiServerURL)
	reverseProxy := httputil.NewSingleHostReverseProxy(u)
	reverseProxy.Transport = h.HTTPClient.Transport

	r.URL.Host = u.Host
	r.URL.Scheme = u.Scheme

	r.Header.Del("Authorization")
	r.Header.Set("X-Remote-User", login)
	for _, group := range groups {
		r.Header.Set("X-Remote-Group", group)
	}
	r.Host = u.Host
	reverseProxy.ServeHTTP(w, r)
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	basicLogin, basicPassword, ok := r.BasicAuth()
	if !ok {
		logger.Error("401 Unauthorized, no basic auth credentials")
		http.Error(w, "Unauthorized", 401)
		return
	}
	groups := h.validateCredentials(basicLogin, basicPassword)
	if len(groups) == 0 {
		logger.Error("403 Forbidden, Crowd authentication problem")
		http.Error(w, "Forbidden", 403)
		return
	}
	logger.Printf("%s %v -- [%s] %s%s %v", basicLogin, groups, r.Method, r.Host, r.RequestURI, r.Header)
	h.modifyRequest(w, r, basicLogin, groups)
}
