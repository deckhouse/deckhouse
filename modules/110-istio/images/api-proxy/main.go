package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	jose "github.com/square/go-jose/v3"
)

type publicMetadata struct {
	ClusterUUID string `json:"clusterUUID,omitempty"`
	AuthnKeyPub string `json:"authnKeyPub,omitempty"`
	RootCA      string `json:"rootCA,omitempty"`
}

// map[custerUUID]pubilcMetadata
type remotePublicMetadata map[string]publicMetadata

type jwtPayload struct {
	Iss   string
	Sub   string
	Aud   string
	Scope string
	Nbf   int64
	Exp   int64
}

var (
	logger             = log.New(os.Stdout, "http: ", log.LstdFlags)
	httpProxyTransport *http.Transport
	probeClient        *http.Client
)

func checkAuthn(header http.Header, scope string) error {
	reqTokenString := header.Get("Authorization")
	if !strings.HasPrefix(reqTokenString, "Bearer ") {
		fmt.Errorf("Bearer authorization required.")
	}
	reqTokenString = strings.TrimPrefix(reqTokenString, "Bearer ")

	reqToken, err := jose.ParseSigned(reqTokenString)
	if err != nil {
		return err
	}
	payloadBytes := reqToken.UnsafePayloadWithoutVerification()

	var payload jwtPayload
	err = json.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return err
	}

	remotePublicMetadataBytes, err := ioutil.ReadFile("/remote/remote-public-metadata.json")
	if err != nil {
		return err
	}

	var remotePublicMetadataMap remotePublicMetadata
	err = json.Unmarshal(remotePublicMetadataBytes, &remotePublicMetadataMap)
	if err != nil {
		return err
	}

	if payload.Aud != os.Getenv("CLUSTER_UUID") {
		return fmt.Errorf("JWT is signed for wrong destination cluster.")
	}

	if payload.Scope != scope {
		return fmt.Errorf("JWT is signed for wrong scope.")
	}

	if payload.Exp < time.Now().UTC().Unix() {
		return fmt.Errorf("JWT token expired.")
	}

	if _, ok := remotePublicMetadataMap[payload.Sub]; !ok {
		return fmt.Errorf("JWT is signed for unknown source cluster.")
	}
	remoteAuthnKeyPubBlock, _ := pem.Decode([]byte(remotePublicMetadataMap[payload.Sub].AuthnKeyPub))
	remoteAuthnKeyPub, err := x509.ParsePKIXPublicKey(remoteAuthnKeyPubBlock.Bytes)
	if err != nil {
		return err
	}

	if _, err := reqToken.Verify(remoteAuthnKeyPub); err != nil {
		return fmt.Errorf("Cannot verify JWT token with known public key.")
	}

	return nil
}

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

	err := checkAuthn(r.Header, "api")
	if err != nil {
		http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
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

	listenCert, err := tls.LoadX509KeyPair("/listen-cert/tls.crt", "/listen-cert/tls.key")
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
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    kubeCertPool,
			Certificates: []tls.Certificate{listenCert},
		},
	}

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
}
