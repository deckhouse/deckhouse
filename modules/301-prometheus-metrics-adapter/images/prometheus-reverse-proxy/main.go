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

package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var logger = log.New(os.Stdout, "http: ", log.LstdFlags)

var PrometheusURL = os.Getenv("PROMETHEUS_URL")

var (
	reNamespaceMatcher      = regexp.MustCompile(`.*namespace="([0-9a-zA-Z_\-]+)".*`)
	reMultiNamespaceMatcher = regexp.MustCompile(`.*namespace=~.*`)
)

var httpTransport http.RoundTripper

const configPath = "/etc/prometheus-reverse-proxy/reverse-proxy.json"

var (
	appliedConfigMtime int64 = 0
	config             map[string]map[string]CustomMetricConfig
)

type CustomMetricConfig struct {
	Cluster    string            `json:"cluster"`
	Namespaced map[string]string `json:"namespaced"`
}

type MetricHandler struct {
	ObjectType    string
	MetricName    string
	Selector      string
	GroupBy       string
	Namespace     string
	MetricConfig  CustomMetricConfig
	QueryTemplate string
}

func initHttpTransport() {
	httpTransport = wrapKubeTransport(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 1 * time.Second,
		}).DialContext,

		TLSHandshakeTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})
}

func (m *MetricHandler) Init() error {
	namespaceMatch := reNamespaceMatcher.FindStringSubmatch(m.Selector)
	if namespaceMatch != nil {
		m.Namespace = namespaceMatch[1]
	} else {
		if reMultiNamespaceMatcher.MatchString(m.Selector) {
			return fmt.Errorf("multiple namespaces are not implemented. Selector: %s", m.Selector)
		} else {
			return fmt.Errorf("no 'namespace=' label in selector '%s' given", m.Selector)
		}
	}

	if metricConfig, ok := config[m.ObjectType][m.MetricName]; ok {
		m.MetricConfig = metricConfig
	} else {
		return fmt.Errorf("metric '%s' for object '%s' not configured", m.MetricName, m.ObjectType)
	}

	if queryTemplate, ok := m.MetricConfig.Namespaced[m.Namespace]; ok {
		m.QueryTemplate = queryTemplate
	} else if len(m.MetricConfig.Cluster) > 0 {
		m.QueryTemplate = m.MetricConfig.Cluster
	} else {
		return fmt.Errorf("metric '%s' for object '%s' not configured for namespace '%s' or cluster-wide", m.MetricName, m.ObjectType, m.Namespace)
	}

	return nil
}

func (m *MetricHandler) RenderQuery() string {
	query := strings.Replace(m.QueryTemplate, "<<.LabelMatchers>>", m.Selector, -1)
	query = strings.Replace(query, "<<.GroupBy>>", m.GroupBy, -1)
	return query
}

func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.String())
}

func httpMyRouter(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.String(), "/api/v1/query?query=custom_metric%3A%3A") {
		httpHandlerCustomMetric(w, r)
	} else {
		httpProxyPass(w, r)
	}
}

func httpHandlerCustomMetric(w http.ResponseWriter, r *http.Request) {
	defer logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.String())

	fStat, _ := os.Stat(configPath)
	if mtime := fStat.ModTime().Unix(); mtime != appliedConfigMtime {
		appliedConfigMtime = mtime
		f, _ := os.Open(configPath)
		defer f.Close()
		json.NewDecoder(f).Decode(&config)
	}

	// query=custom_query::<ObjectType>::<MetricName>::<Selector>::<GroupBy>
	args := strings.Split(r.URL.Query().Get("query"), "::")
	metricHandler := &MetricHandler{
		ObjectType: args[1],
		MetricName: args[2],
		Selector:   args[3],
		GroupBy:    args[4],
	}

	err := metricHandler.Init()
	if err != nil {
		logger.Println("ERROR", err)
		http.Error(w, "Internal error. "+err.Error(), http.StatusInternalServerError)
		return
	}

	prometheusQuery := metricHandler.RenderQuery()

	u, _ := url.Parse(PrometheusURL)
	u.Path = r.URL.Path

	q := r.URL.Query()
	q.Set("query", prometheusQuery)
	u.RawQuery = q.Encode()

	client := &http.Client{Transport: httpTransport, Timeout: time.Minute}
	resp, err := client.Get(u.String())
	defer resp.Body.Close()

	if err != nil {
		logger.Println("ERROR", err)
		http.Error(w, "Internal error. "+err.Error(), http.StatusInternalServerError)
	}

	if len(resp.Header.Get("Content-Type")) > 0 {
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	}
	if len(resp.Header.Get("Content-Length")) > 0 {
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	}
	io.Copy(w, resp.Body)
}

func httpProxyPass(w http.ResponseWriter, r *http.Request) {
	defer logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.String())

	u, _ := url.Parse(PrometheusURL)
	reverseProxy := httputil.NewSingleHostReverseProxy(u)
	reverseProxy.Transport = httpTransport
	reverseProxy.ServeHTTP(w, r)
}

func main() {
	listenAddr := "0.0.0.0:8000"

	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		logger.Fatalf("Config file %s does not exist", configPath)
	}

	initHttpTransport()

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/", http.HandlerFunc(httpMyRouter))

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
}

const (
	renewTokenPeriod = 30 * time.Second
	tokenPath        = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// Update token periodically because BoundServiceAccountToken feature is enabled for Kubernetes >=1.21
// https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#bound-service-account-token-volume

type kubeTransport struct {
	mu     sync.RWMutex
	token  string
	expiry time.Time

	base http.RoundTripper
}

func wrapKubeTransport(base http.RoundTripper) http.RoundTripper {
	t := &kubeTransport{base: base}
	t.updateToken()
	return t
}

func (t *kubeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.updateToken()

	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+t.GetToken())

	return t.base.RoundTrip(r2)
}

func (t *kubeTransport) updateToken() {
	t.mu.RLock()
	exp := t.expiry
	t.mu.RUnlock()

	now := time.Now()
	if now.Before(exp) {
		// Do not need to update token yet
		return
	}

	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		logger.Println("cannot read service account token, will try later")
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = string(token)
	t.expiry = now.Add(renewTokenPeriod)
}

func (t *kubeTransport) GetToken() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.token
}
