/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/go-jose/go-jose/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"
)

type Proxy struct {
	serverCert                           *tls.Certificate
	probeClient                          *http.Client
	httpProxyTransport                   *http.Transport
	reverseProxy                         *httputil.ReverseProxy
	clientSet                            *kubernetes.Clientset
	lwRemoteClustersPublicMetadata       *cache.ListWatch
	remoteClustersPublicMetadataInformer cache.SharedInformer
}

func NewProxy(namespace string) (*Proxy, error) {

	// Create config for Kubernetes-client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	// Create client Kubernetes
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	lwRemoteClustersPublicMetadata := cache.NewFilteredListWatchFromClient(
		clientSet.CoreV1().RESTClient(),
		"secrets",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=d8-remote-clusters-public-metadata"
		},
	)

	serverCert, err := generateListenCert()
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	httpProxyTransport, err := initProxyTransport()
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	httpProxyClient, err := initProxyClient(httpProxyTransport)
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	proxy := &Proxy{
		clientSet:                      clientSet,
		serverCert:                     &serverCert,
		probeClient:                    httpProxyClient,
		httpProxyTransport:             httpProxyTransport,
		lwRemoteClustersPublicMetadata: lwRemoteClustersPublicMetadata,
	}

	reverse, err := proxy.NewReverseProxyHTTP()
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] error creating reverse proxy: %w", err)
	}

	proxy.reverseProxy = reverse

	return proxy, nil
}

// ingress controller doesn't authenticate proxy for now
func generateListenCert() (tls.Certificate, error) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "istio-api-proxy",
		},
		DNSNames: []string{"api-proxy", "api-proxy.d8-istio", "api-proxy.d8-istio.svc"},

		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
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

func initProxyTransport() (*http.Transport, error) {
	kubeCA, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("[api-proxy] Error : %w", err)
	}

	kubeCertPool := x509.NewCertPool()

	if ok := kubeCertPool.AppendCertsFromPEM(kubeCA); !ok {
		return nil, fmt.Errorf("[api-proxy] failed to append CA certificates")
	}

	httpProxyTransport := &http.Transport{
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

	return httpProxyTransport, nil
}

func initProxyClient(httpProxyTransport *http.Transport) (*http.Client, error) {
	// for readiness healthcheck
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: httpProxyTransport,
	}, nil
}

func (p *Proxy) CheckAuthn(header http.Header, scope string) error {
	reqTokenString := header.Get("Authorization")
	if !strings.HasPrefix(reqTokenString, "Bearer ") {
		return fmt.Errorf("bearer authorization required")
	}
	reqTokenString = strings.TrimPrefix(reqTokenString, "Bearer ")

	reqToken, err := jose.ParseSigned(reqTokenString)
	if err != nil {
		return err
	}
	payloadBytes := reqToken.UnsafePayloadWithoutVerification()

	var payload JwtPayload
	err = json.Unmarshal(payloadBytes, &payload)
	if err != nil {
		return err
	}

	// Load remote-public-metadata.json
	remotePublicMetadataMap, err := p.extractRemotePublicMetadata()
	if err != nil {
		return fmt.Errorf("[api-proxy] Error : %w", err)
	}

	if payload.Aud != os.Getenv("CLUSTER_UUID") {
		return fmt.Errorf("[api-proxy] JWT is signed for wrong destination cluster")
	}

	if payload.Scope != scope {
		return fmt.Errorf("[api-proxy] JWT is signed for wrong scope")
	}

	if payload.Exp < time.Now().UTC().Unix() {
		return fmt.Errorf("[api-proxy] JWT token expired")
	}

	if _, ok := remotePublicMetadataMap[payload.Sub]; !ok {
		return fmt.Errorf("[api-proxy] JWT is signed for unknown source cluster")
	}
	remoteAuthnKeyPubBlock, rests := pem.Decode([]byte(remotePublicMetadataMap[payload.Sub].AuthnKeyPub))
	if remoteAuthnKeyPubBlock == nil {
		return fmt.Errorf("[api-proxy] remote authn public key is invalid")
	}
	if len(rests) > 0 {
		return fmt.Errorf("[api-proxy] remote authn public key is invalid")
	}

	remoteAuthnKeyPub, err := x509.ParsePKIXPublicKey(remoteAuthnKeyPubBlock.Bytes)
	if err != nil {
		return err
	}

	if _, err := reqToken.Verify(remoteAuthnKeyPub); err != nil {
		return fmt.Errorf("[api-proxy] cannot verify JWT token with known public key")
	}

	return nil
}

func (p *Proxy) NewReverseProxyHTTP() (*httputil.ReverseProxy, error) {


	proxyDirector := func(req *http.Request) {
		// impersonate as current ServiceAccount
		saToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			logger.Printf("[api-proxy] Error reading SA token: %v", err)
		}
		
		req.Header.Del("Authorization")
		req.Header.Add("Authorization", "Bearer "+string(saToken))
		req.URL.Scheme = "https"
		req.URL.Host = "kubernetes.default.svc." + os.Getenv("CLUSTER_DOMAIN")
	}

	reverse := &httputil.ReverseProxy{
		Director:      proxyDirector,
		Transport:     p.httpProxyTransport,
		ErrorLog:      logger,
		FlushInterval: 50 * time.Millisecond,
		ModifyResponse: func(resp *http.Response) error {
			logger.Println("[apiserver]", resp.Status)
			return nil
		},
	}

	return reverse, nil
}

// ExtractRemotePublicMetadata extract remote-public-metadata.json fom Secret d8-remote-clusters-public-metadata
func (p *Proxy) extractRemotePublicMetadata() (RemotePublicMetadata, error) {

	items := p.remoteClustersPublicMetadataInformer.GetStore().List()
	if len(items) == 0 {
		return nil, fmt.Errorf("no secrets found in d8-remote-clusters-public-metadata")
	}

	secret, ok := items[0].(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("failed to cast item to *v1.Secret")
	}

	data, exists := secret.Data["remote-public-metadata.json"]
	if !exists {
		return nil, fmt.Errorf("secret d8-remote-clusters-public-metadata does not contain remote-public-metadata.json")
	}

	var metadata RemotePublicMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse remote-public-metadata.json: %w", err)
	}

	return metadata, nil
}

func (p *Proxy) Watch(ctx context.Context) error {
	p.remoteClustersPublicMetadataInformer = cache.NewSharedInformer(
		p.lwRemoteClustersPublicMetadata,
		&v1.Secret{},
		0,
	)
	go func() {
		defer fmt.Println("[INFO] [api-proxy] Watcher stopped")
		p.remoteClustersPublicMetadataInformer.Run(ctx.Done())
	}()

	if !cache.WaitForCacheSync(ctx.Done(),
		p.remoteClustersPublicMetadataInformer.HasSynced) {
		return fmt.Errorf("[api-proxy] Failed to sync caches")
	}

	return nil
}
