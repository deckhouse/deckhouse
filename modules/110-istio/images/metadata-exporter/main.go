package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"
)

var certPath = "/certs/root-cert.pem"
var logger = log.New(os.Stdout, "http: ", log.LstdFlags)

type spiffeKey struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	E   string   `json:"e"`
	N   string   `json:"n"`
	X5c [][]byte `json:"x5c"`
}

type spiffeEndpoint struct {
	SpiffeSequence    int         `json:"spiffe_sequence"`
	SpiffeRefreshHint int         `json:"spiffe_refresh_hint"`
	Keys              []spiffeKey `json:"keys"`
}

var spiffeBundleJSON string
func renderSpiffeBundleJSON() {
	pubPem, err := ioutil.ReadFile(certPath)
	if err != nil {
		panic("Cert file read error: " + err.Error())
	}

	pubPemBlock, _ := pem.Decode(pubPem)
	if pubPemBlock == nil {
		panic("PEM decode error")
	}

	cert, err := x509.ParseCertificate(pubPemBlock.Bytes)
	if err != nil {
		panic("x509 parse error: " + err.Error())
	}

	rsaPublicKey := cert.PublicKey.(*rsa.PublicKey)
	n := base64.RawURLEncoding.EncodeToString(rsaPublicKey.N.Bytes())

	x5c := make([][]byte, 0)
	x5c = append(x5c, pubPemBlock.Bytes)

	sk := spiffeKey{
		Kty: "RSA",
		Use: "x509-svid",
		E:   "AQAB",
		N:   n,
		X5c: x5c,
	}

	keys := []spiffeKey{sk}
	se := spiffeEndpoint{
		SpiffeSequence:    1,
		SpiffeRefreshHint: 2419200,
		Keys:              keys,
	}

	jsonbuf, err := json.MarshalIndent(se, "", "  ")
	spiffeBundleJSON = string(jsonbuf)
}

//goland:noinspection SpellCheckingInspection
func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerFederationServices(w http.ResponseWriter, r *http.Request) {
	verify := r.Header.Get("ssl-client-verify")
	subject := r.Header.Get("ssl-client-subject-dn")
	matched, _ := regexp.Match(`(^|,)CN=deckhouse(,|$)`,[]byte(subject))
	if verify != "SUCCESS" || matched != true {
		http.Error(w, "Proper client certificate with CN=deckhouse wasn't provided.", http.StatusUnauthorized)
		return
	}

	data, err := ioutil.ReadFile("/metadata/services.json")
	if err != nil {
		http.Error(w, "Error reading services.json", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, string(data))
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerFederationIngressgateways(w http.ResponseWriter, r *http.Request) {
	verify := r.Header.Get("ssl-client-verify")
	subject := r.Header.Get("ssl-client-subject-dn")
	matched, _ := regexp.Match(`(^|,)CN=deckhouse(,|$)`,[]byte(subject))
	if verify != "SUCCESS" || matched != true {
		http.Error(w, "Proper client certificate with CN=deckhouse wasn't provided.", http.StatusUnauthorized)
		return
	}

	data, err := ioutil.ReadFile("/metadata/ingressgateways.json")
	if err != nil {
		http.Error(w, "Error reading ingressgateways.json", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, string(data))
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerMulticlusterAPIHost(w http.ResponseWriter, r *http.Request) {
	verify := r.Header.Get("ssl-client-verify")
	subject := r.Header.Get("ssl-client-subject-dn")
	matched, _ := regexp.Match(`(^|,)CN=deckhouse(,|$)`,[]byte(subject))
	if verify != "SUCCESS" || matched != true {
		http.Error(w, "Proper client certificate with CN=deckhouse wasn't provided.", http.StatusUnauthorized)
		return
	}

	apiHost := os.Getenv("MULTICLUSTER_API_HOST")
	if len(apiHost) == 0 {
		http.Error(w, "Error reading api host", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, apiHost)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerSpiffeBundleEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
		return
	}

	fmt.Fprint(w, spiffeBundleJSON)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerRootCert(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile(certPath)
	if err != nil {
		http.Error(w, "Error reading " + certPath, http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, string(data))
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func main() {
	renderSpiffeBundleJSON()

	listenAddr := "0.0.0.0:8080"

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/metadata/public/spiffe-bundle-endpoint", http.HandlerFunc(httpHandlerSpiffeBundleEndpoint))
	router.Handle("/metadata/public/root-cert.pem", http.HandlerFunc(httpHandlerRootCert))

	if os.Getenv("FEDERATION_ENABLED") == "true" {
		router.Handle("/metadata/private/federation-services", http.HandlerFunc(httpHandlerFederationServices))
		router.Handle("/metadata/private/federation-ingressgateways", http.HandlerFunc(httpHandlerFederationIngressgateways))
	}
	if os.Getenv("MULTICLUSTER_ENABLED") == "true" {
		router.Handle("/metadata/private/multicluster-api-host", http.HandlerFunc(httpHandlerMulticlusterAPIHost))
	}

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
