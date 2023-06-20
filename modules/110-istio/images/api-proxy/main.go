/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
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

	remotePublicMetadataBytes, err := os.ReadFile("/remote/remote-public-metadata.json")
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
	kubeCA, _ := os.ReadFile("/run/secrets/kubernetes.io/serviceaccount/ca.crt")
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
	if len(r.TLS.PeerCertificates) == 0 {
		errstring := "Only requests with client certificate are allowed."
		http.Error(w, errstring, http.StatusUnauthorized)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
		return
	}

	if r.TLS.PeerCertificates[0].Subject.Organization[0] != "ingress-nginx:auth" {
		errstring := "Only requests from ingress are allowed."
		http.Error(w, errstring, http.StatusUnauthorized)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
		return
	}

	err := checkAuthn(r.Header, "api")
	if err != nil {
		errstring := "Authentication error: " + err.Error()
		http.Error(w, errstring, http.StatusUnauthorized)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
		return
	}

	// impersonate as current ServiceAccount
	saToken, _ := os.ReadFile("/run/secrets/kubernetes.io/serviceaccount/token")
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
		ModifyResponse: func(resp *http.Response) error {
			logger.Println("[apiserver]", resp.Status)
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

// ingress controller doesn't authenticate proxy for now
func generateListenCert() (tls.Certificate, error) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName: "istio-api-proxy",
		},
		DNSNames: []string{"api-proxy", "api-proxy.d8-istio", "api-proxy.d8-istio.svc"},

		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, cert, &certPrivKey.PublicKey, certPrivKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		return tls.Certificate{}, err
	}

	return serverCert, nil
}

func main() {
	listenAddr := "0.0.0.0:4443"

	initProxyTransport()

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/ready", http.HandlerFunc(httpHandlerReady))
	router.Handle("/", http.HandlerFunc(httpHandlerApiProxy))

	kubeCA, _ := os.ReadFile("/etc/ssl/kube-rbac-proxy-ca.crt")
	kubeCertPool := x509.NewCertPool()
	kubeCertPool.AppendCertsFromPEM(kubeCA)

	listenCert, err := generateListenCert()
	if err != nil {
		logger.Fatalf("Could not generate server certificates on: %v\n", err)
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
