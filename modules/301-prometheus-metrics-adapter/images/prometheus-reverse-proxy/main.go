package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var logger = log.New(os.Stdout, "http: ", log.LstdFlags)
var PROMETHEUS_URL = os.Getenv("PROMETHEUS_URL")

var reNamespaceMatcher = regexp.MustCompile(`.*namespace="([0-9a-zA-Z_\-]+)".*`)
var reMultiNamespaceMatcher = regexp.MustCompile(`.*namespace=~.*`)

var sslClientCrt = "/etc/ssl/prometheus-api-client-tls/tls.crt"
var sslClientKey = "/etc/ssl/prometheus-api-client-tls/tls.key"
var httpTransport *http.Transport

const configPath = "/etc/prometheus-reverse-proxy/reverse-proxy.json"

var appliedConfigMtime int64 = 0
var config map[string]map[string]CustomMetricConfig

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
	sslClient, _ := tls.LoadX509KeyPair(sslClientCrt, sslClientKey)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{sslClient},
		InsecureSkipVerify: true,
	}

	httpTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 1 * time.Second,
		}).DialContext,

		TLSHandshakeTimeout: 1 * time.Second,
		TLSClientConfig:     tlsConfig,
	}
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

func http_handler_healthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.String())
}

func http_my_router(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.String(), "/api/v1/query?query=custom_metric%3A%3A") {
		http_handler_custom_metric(w, r)
	} else {
		http_proxy_pass(w, r)
	}
}

func http_handler_custom_metric(w http.ResponseWriter, r *http.Request) {
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

	u, _ := url.Parse(PROMETHEUS_URL)
	u.Path = r.URL.Path

	q := r.URL.Query()
	q.Set("query", prometheusQuery)
	u.RawQuery = q.Encode()

	client := &http.Client{Transport: httpTransport}
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

func http_proxy_pass(w http.ResponseWriter, r *http.Request) {
	defer logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.String())

	u, _ := url.Parse(PROMETHEUS_URL)
	reverseProxy := httputil.NewSingleHostReverseProxy(u)
	reverseProxy.Transport = httpTransport
	reverseProxy.ServeHTTP(w, r)
}

func main() {
	listenAddr := "0.0.0.0:8000"

	initHttpTransport()

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(http_handler_healthz))
	router.Handle("/", http.HandlerFunc(http_my_router))

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
