/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// This type implements the http.RoundTripper interface
type LoggingRoundTripper struct {
	Proxied http.RoundTripper
}

func (lrt LoggingRoundTripper) RoundTrip(req *http.Request) (res *http.Response, e error) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		log.Error(err)
	}
	// Send the request, get the response (or the error)
	res, e = lrt.Proxied.RoundTrip(req)

	log.Infof("%s", string(dump))
	if res != nil {
		log.Infof("response: %s", res.Status)
	}
	return
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

	var config config
	err := config.getEnvConfig()
	if err != nil {
		log.Fatal(err)
	}

	proxy := newMadisonProxy(config)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", readyHandler)
	mux.HandleFunc("/", proxy.ServeHTTP)

	s := &http.Server{
		Addr:    config.ListenHost + ":" + config.ListenPort,
		Handler: mux,
	}
	go func() {
		err := s.ListenAndServe()
		if err == nil || err == http.ErrServerClosed {
			log.Info("Shutting down.")
			return
		}
		log.Error(err)
	}()

	// Block to wait for a signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigs

	// 30 sec is the readiness check timeout
	deadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	log.Info("Got signal ", sig)
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
	return &httputil.ReverseProxy{
		Transport: LoggingRoundTripper{transport},
		Director: func(req *http.Request) {
			req.URL.Scheme = c.MadisonScheme
			req.URL.Host = c.MadisonBackend

			switch req.URL.Path {
			case "/api/v1/alerts", "/api/v2/alerts":
				req.URL.Path = "/api/events/prometheus/" + c.MadisonAuthKey
			case "/readyz":
				req.URL.Path = "/healthz"
			default:
				log.Fatalf("path %q is not allowed", req.URL.Path)
			}

			req.Host = c.MadisonHost
			req.Header.Set("Host", c.MadisonHost)
		},
	}
}
