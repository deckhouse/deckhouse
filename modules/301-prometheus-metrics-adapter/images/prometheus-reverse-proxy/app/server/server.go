/*
Copyright 2022 Flant JSC

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

package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

func initHttpTransport() http.RoundTripper {
	httpTransport := http.RoundTripper(&http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: 1 * time.Second,
		}).DialContext,

		TLSHandshakeTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	httpTransport = wrapKubeTransport(httpTransport)
	return httpTransport
}

type Server struct {
	listenAddr string

	PrometheusURL *url.URL
	Transport     http.RoundTripper

	Client    *http.Client
	ProxyPass *httputil.ReverseProxy
}

func NewServer() *Server {
	listenAddr := os.Getenv("PROXY_LISTEN_ADDRESS")
	if listenAddr == "" {
		listenAddr = "0.0.0.0:8000"
	}

	promURL, _ := url.Parse(os.Getenv("PROMETHEUS_URL"))
	transport := initHttpTransport()

	proxy := httputil.NewSingleHostReverseProxy(promURL)
	proxy.Transport = transport

	httpClient := &http.Client{Transport: transport, Timeout: time.Minute}

	return &Server{
		listenAddr:    listenAddr,
		PrometheusURL: promURL,
		Client:        httpClient,
		ProxyPass:     proxy,
	}
}

func (s *Server) Listen() {
	infLog.Println("PROMETHEUS_URL=", s.PrometheusURL.String(), "...")
	infLog.Println("server is starting to listen on ", s.listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/", wrapLoggerHandler(s.router))

	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Ok.")
	})

	server := &http.Server{
		Addr:         s.listenAddr,
		Handler:      router,
		ErrorLog:     errLog,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errLog.Fatalf("could not listen on %s: %v\n", s.listenAddr, err)
	}
}

func (s *Server) router(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.String(), "/api/v1/query?query=custom_metric%3A%3A") {
		s.handlerCustomMetric(w, r)
	} else {
		s.ProxyPass.ServeHTTP(w, r)
	}
}

func (s *Server) handlerCustomMetric(w http.ResponseWriter, r *http.Request) {
	reqID := r.Context().Value("id").(string)

	const queryArgsNumber = 5

	// query=custom_query::<ObjectType>::<MetricName>::<Selector>::<GroupBy>
	args := strings.Split(r.URL.Query().Get("query"), "::")
	if len(args) != queryArgsNumber {
		err := fmt.Errorf("query must container %d args, got %d", queryArgsNumber, len(args))
		errLog.Printf("%s -- %s\n", reqID, err)
		http.Error(w, "Internal error. "+err.Error(), http.StatusInternalServerError)
		return
	}

	metricHandler := &MetricHandler{
		ObjectType: args[1],
		MetricName: args[2],
		Selector:   args[3],
		GroupBy:    args[4],
	}

	err := metricHandler.Init()
	if err != nil {
		errLog.Printf("%s -- %s\n", reqID, err)
		http.Error(w, "Internal error. "+err.Error(), http.StatusInternalServerError)
		return
	}

	prometheusQuery := metricHandler.RenderQuery()

	newURL := *s.PrometheusURL
	newURL.Path = r.URL.Path

	q := r.URL.Query()
	q.Set("query", prometheusQuery)
	newURL.RawQuery = q.Encode()

	resp, err := s.Client.Get(newURL.String())
	defer resp.Body.Close()

	if err != nil {
		errLog.Printf("%s -- %s\n", reqID, err)
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
