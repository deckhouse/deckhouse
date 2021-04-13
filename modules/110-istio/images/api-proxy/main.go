package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"time"
)

var (
	logger             = log.New(os.Stdout, "http: ", log.LstdFlags)
	httpProxyTransport *http.Transport
	probeClient        *http.Client
)

func initProxyTransport() {
	kubeCA, _ := ioutil.ReadFile("/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	kubeCertPool := x509.NewCertPool()
	kubeCertPool.AppendCertsFromPEM(kubeCA)

	httpProxyTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 30 * time.Second,
		}).DialContext,

		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			RootCAs:            kubeCertPool,
		},

		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	// for readiness healthcheck
	probeClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: httpProxyTransport,
	}
}

func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerReady(w http.ResponseWriter, r *http.Request) {
	_, err := probeClient.Get("https://kubernetes.default.svc." + os.Getenv("CLUSTER_DOMAIN") + "/version")
	if err != nil {
		logger.Fatalf("Readiness probe error: %v\n", err)
		http.Error(w, "Error", http.StatusInternalServerError)
	} else {
		fmt.Fprint(w, "Ok.")
	}
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerApiProxy(w http.ResponseWriter, r *http.Request) {
	// check if request was passed by ingress
	if r.TLS.PeerCertificates[0].Subject.Organization[0] != "ingress-nginx:auth" {
		http.Error(w, "Only requests from ingress are allowed.", http.StatusUnauthorized)
		return
	}

	// check if original request (from remote istiod) was authenticated by ingress
	// and original client cert was with right CN
	subject := r.Header.Get("ssl-client-subject-dn")
	if matched, _ := regexp.Match(`(^|,)CN=deckhouse(,|$)`, []byte(subject)); !matched {
		http.Error(w, "Proper client certificate with CN=deckhouse wasn't provided.", http.StatusUnauthorized)
		return
	}

	// impersonate as current ServiceAccount
	saToken, _ := ioutil.ReadFile("/run/secrets/kubernetes.io/serviceaccount/token")
	proxyDirector := func(req *http.Request) {
		req.Header.Del("Authorization")
		req.Header.Add("Authorization", "Bearer "+string(saToken))
		req.URL.Scheme = "https"
		req.URL.Host = "kubernetes.default.svc." + os.Getenv("CLUSTER_DOMAIN")
	}

	proxy := &httputil.ReverseProxy{
		Director:      proxyDirector,
		Transport:     httpProxyTransport,
		ErrorLog:      logger,
		FlushInterval: 50 * time.Millisecond,
	}

	proxy.ServeHTTP(w, r)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func main() {
	listenAddr := "0.0.0.0:4443"

	initProxyTransport()

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/ready", http.HandlerFunc(httpHandlerReady))
	router.Handle("/", http.HandlerFunc(httpHandlerApiProxy))

	kubeCA, _ := ioutil.ReadFile("/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	kubeCertPool := x509.NewCertPool()
	kubeCertPool.AppendCertsFromPEM(kubeCA)

	listenCert, err := tls.LoadX509KeyPair("/listen-cert/tls.crt","/listen-cert/tls.key")
	if err != nil {
		logger.Fatalf("Could not load server certificates on: %v\n", err)
	}

	server := &http.Server{
		Addr:     listenAddr,
		Handler:  router,
		ErrorLog: logger,
		TLSConfig: &tls.Config{
			// Allow unauthenticated requests to probes, but check certificates from ingress.
			// Additional subject check is in httpHandlerApiProxy func.
			ClientAuth: tls.VerifyClientCertIfGiven,
			ClientCAs: kubeCertPool,
			Certificates: []tls.Certificate{listenCert},
		},
	}

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	if err := server.ListenAndServeTLS("",""); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
}
