/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	jose "github.com/square/go-jose/v3"
)

var rootCAPath = "/certs/root-cert.pem"
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

// TODO import from hooks package
// Warning! These structs are duplicated in hooks/private/crd
type AlliancePublicMetadata struct {
	ClusterUUID string `json:"clusterUUID,omitempty"`
	AuthnKeyPub string `json:"authnKeyPub,omitempty"`
	RootCA      string `json:"rootCA,omitempty"`
}

type FederationPrivateMetadata struct {
	IngressGateways *[]struct {
		Address string `json:"address"`
		Port    uint   `json:"port"`
	} `json:"ingressGateways"`
	PublicServices *[]struct {
		Hostname string `json:"hostname"`
		Ports    []struct {
			Name string `json:"name"`
			Port uint   `json:"port"`
		} `json:"ports"`
	} `json:"publicServices"`
}

type MulticlusterPrivateMetadata struct {
	IngressGateways *[]struct {
		Address string `json:"address"`
		Port    uint   `json:"port"`
	} `json:"ingressGateways"`
	APIHost     string `json:"apiHost,omitempty"`
	NetworkName string `json:"networkName,omitempty"`
}

// map[custerUUID]pubilcMetadata
type remotePublicMetadata map[string]AlliancePublicMetadata

type jwtPayload struct {
	Iss   string
	Sub   string
	Aud   string
	Scope string
	Nbf   int64
	Exp   int64
}

var spiffeBundleJSON string
var publicMetadataJSON string

func renderSpiffeBundleJSON() {
	pubPem, err := os.ReadFile(rootCAPath)
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
	if err != nil {
		panic("Error Marshall spiffe endpoint json: " + err.Error())
	}

	spiffeBundleJSON = string(jsonbuf)
}

func renderPublicMetadataJSON() {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	if len(clusterUUID) == 0 {
		panic("Error reading cluster UUID")
	}

	authnKeyPubPem, err := os.ReadFile("/keys/pub.pem")
	if err != nil {
		panic("pub key file read error: " + err.Error())
	}

	rootCAPem, err := os.ReadFile(rootCAPath)
	if err != nil {
		panic("root ca file read error: " + err.Error())
	}

	pm := AlliancePublicMetadata{
		ClusterUUID: clusterUUID,
		AuthnKeyPub: string(authnKeyPubPem),
		RootCA:      string(rootCAPem),
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster public metadata to json: " + err.Error())
	}

	publicMetadataJSON = string(jsonbuf)
}

func renderFederationPrivateMetadataJSON() string {
	var pm FederationPrivateMetadata

	data, err := os.ReadFile("/metadata/ingressgateways-array.json")
	if err == nil {
		json.Unmarshal(data, &pm.IngressGateways)
	}

	if os.Getenv("FEDERATION_ENABLED") == "true" {
		data, err := os.ReadFile("/metadata/services-array.json")
		if err == nil {
			json.Unmarshal(data, &pm.PublicServices)
		}
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster private metadata to json: " + err.Error())
	}
	return string(jsonbuf)
}

func renderMulticlusterPrivateMetadataJSON() string {
	var pm MulticlusterPrivateMetadata

	data, err := os.ReadFile("/metadata/ingressgateways-array.json")
	if err == nil {
		json.Unmarshal(data, &pm.IngressGateways)
	}

	pm.NetworkName = os.Getenv("MULTICLUSTER_NETWORK_NAME")
	if len(pm.NetworkName) == 0 {
		panic("Error reading MULTICLUSTER_NETWORK_NAME from env")
	}

	pm.APIHost = os.Getenv("MULTICLUSTER_API_HOST")
	if len(pm.APIHost) == 0 {
		panic("Error reading MULTICLUSTER_API_HOST from env")
	}

	jsonbuf, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		panic("Error marshalling cluster private metadata to json: " + err.Error())
	}
	return string(jsonbuf)
}

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

func httpHandlerPubilcJSON(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, publicMetadataJSON)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerFederationPrivateJSON(w http.ResponseWriter, r *http.Request) {
	err := checkAuthn(r.Header, "private-federation")
	if err != nil {
		http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
		return
	}

	privateMetadataJSON := renderFederationPrivateMetadataJSON()
	fmt.Fprint(w, privateMetadataJSON)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerMulticlusterPrivateJSON(w http.ResponseWriter, r *http.Request) {
	err := checkAuthn(r.Header, "private-multicluster")
	if err != nil {
		http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
		return
	}

	privateMetadataJSON := renderMulticlusterPrivateMetadataJSON()
	fmt.Fprint(w, privateMetadataJSON)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerSpiffeBundleEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, spiffeBundleJSON)
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

//goland:noinspection SpellCheckingInspection
func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func renderScheduler() {
	time.Sleep(1 * time.Minute)
	renderSpiffeBundleJSON()
	renderPublicMetadataJSON()
}

func main() {
	renderSpiffeBundleJSON()
	renderPublicMetadataJSON()
	go renderScheduler()

	listenAddr := "0.0.0.0:8080"

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/metadata/public/spiffe-bundle-endpoint", http.HandlerFunc(httpHandlerSpiffeBundleEndpoint))
	router.Handle("/metadata/public/public.json", http.HandlerFunc(httpHandlerPubilcJSON))

	if os.Getenv("FEDERATION_ENABLED") == "true" {
		router.Handle("/metadata/private/federation.json", http.HandlerFunc(httpHandlerFederationPrivateJSON))
	}
	if os.Getenv("MULTICLUSTER_ENABLED") == "true" {
		router.Handle("/metadata/private/multicluster.json", http.HandlerFunc(httpHandlerMulticlusterPrivateJSON))
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
