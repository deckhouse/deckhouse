/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var _ http.RoundTripper = LoggingRoundTripper{}

type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Error(err)
	}

	res, e := lrt.Proxied.RoundTrip(req)

	if res != nil {
		log.Infof("request: %s, response: %d", string(dump), res.StatusCode)
	} else {
		log.Infof("request: %s, no response", string(dump))
	}

	return res, e
}

type PathCheckRoundTripper struct {
	Proxied http.RoundTripper
}

func (prt PathCheckRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case
		strings.Contains(req.URL.Path, "/api/events/prometheus"),
		req.URL.Path == "/healthz":
		// Do nothing
	default:
		return nil, errors.New(fmt.Sprintf("path %q is not allowed ", req.URL.Path))
	}

	return prt.Proxied.RoundTrip(req)
}

type config struct {
	ListenHost     string
	ListenPort     string
	MadisonHost    string
	MadisonScheme  string
	MadisonBackend string
	MadisonAuthKey string
}

func (c *config) getEnvConfig() error {
	c.ListenHost = os.Getenv("LISTEN_HOST")
	if c.ListenHost == "" {
		c.ListenHost = "0.0.0.0"
	}

	c.ListenPort = os.Getenv("LISTEN_PORT")
	if c.ListenPort == "" {
		c.ListenPort = "8080"
	}

	c.MadisonHost = os.Getenv("MADISON_HOST")
	if c.MadisonHost == "" {
		c.MadisonHost = "madison.flant.com"
	}

	c.MadisonScheme = os.Getenv("MADISON_SCHEME")
	if c.MadisonScheme == "" {
		c.MadisonScheme = "https"
	}

	c.MadisonBackend = os.Getenv("MADISON_BACKEND")
	if c.MadisonBackend == "" {
		return errors.New("MADISON_BACKEND is not set")
	}

	c.MadisonAuthKey = os.Getenv("MADISON_AUTH_KEY")
	if c.MadisonAuthKey == "" {
		return errors.New("MADISON_AUTH_KEY is not set")
	}
	return nil
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	cfg := config{}

	err := cfg.getEnvConfig()
	if err != nil {
		log.Fatal(err)
	}

	proxy := newMadisonProxy(cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", readyHandler)
	mux.HandleFunc("/", proxy.ServeHTTP)

	s := &http.Server{
		Addr:    net.JoinHostPort(cfg.ListenHost, cfg.ListenPort),
		Handler: mux,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	doneCh := make(chan struct{})

	go func() {
		err := s.ListenAndServe()
		close(doneCh)
		if err == nil || err == http.ErrServerClosed {
			return
		}

		log.Error(err)
	}()

	// Block to wait for a signal
	select {
	case sig := <-sigCh:
		log.Info("Got signal ", sig)
	case <-doneCh:
		log.Info("Shutting down.")
	}

	// 30 sec is the readiness check timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = s.Shutdown(ctx)
	if err != nil {
		log.Error(err)
	}
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newMadisonProxy(c config) http.Handler {
	transport := http.DefaultTransport
	transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	// Disable HTTP2 to avoid the following error:
	//
	// http: proxy error: http2: Transport: cannot retry err
	// [http2: Transport received Server's graceful shutdown GOAWAY] after Request.Body was written;
	// define Request.GetBody to avoid this error
	transport.(*http.Transport).TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	transport.(*http.Transport).ResponseHeaderTimeout = 10 * time.Second

	return &httputil.ReverseProxy{
		Transport: PathCheckRoundTripper{LoggingRoundTripper{transport}},
		Director: func(req *http.Request) {
			req.URL.Scheme = c.MadisonScheme
			req.URL.Host = c.MadisonBackend

			switch req.URL.Path {
			case "/api/v1/alerts", "/api/v2/alerts":
				req.URL.Path = "/api/events/prometheus/" + c.MadisonAuthKey
			case "/readyz":
				req.URL.Path = "/healthz"
			}

			req.Host = c.MadisonHost
			req.Header.Set("Host", c.MadisonHost)
		},
	}
}
