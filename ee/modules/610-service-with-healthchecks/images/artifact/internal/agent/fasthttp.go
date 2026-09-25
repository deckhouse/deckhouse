/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
	corev1 "k8s.io/api/core/v1"
)

const (
	maxIdleConnDuration = 100 * time.Second
	// defaultTimeout mirrors the CRD default for timeoutSeconds. It only applies to a probe target
	// built from a spec that predates that default, since a zero timeout would let fasthttp wait
	// forever and stall the probe worker.
	defaultTimeout = time.Second
)

// clientKey identifies one probe configuration.
//
// fasthttp.Client copies ReadTimeout, WriteTimeout and TLSConfig into a per-host client the first
// time it dials a given host and never refreshes them afterwards. Mutating a client between
// requests is therefore silently ineffective, and a client shared by probes with different settings
// serves every one of them with whichever settings happened to dial first. Keeping one client per
// distinct configuration is what makes these settings take effect at all; the number of keys is
// bounded by the probe definitions in the cluster, not by the number of probe runs.
type clientKey struct {
	timeoutSeconds        int32
	insecureSkipTLSVerify bool
	caCert                string
	serverName            string
}

var (
	clientsMu sync.Mutex
	clients   = make(map[clientKey]*fasthttp.Client)
)

// clientFor returns the client shared by every probe with this configuration, building it on first
// use. A malformed caCert fails here rather than at dial time, so the probe reports the
// configuration error instead of a TLS handshake failure.
func clientFor(key clientKey) (*fasthttp.Client, error) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	if client, ok := clients[key]; ok {
		return client, nil
	}

	tlsConfig, err := key.tlsConfig()
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(key.timeoutSeconds) * time.Second
	if key.timeoutSeconds <= 0 {
		timeout = defaultTimeout
	}

	client := &fasthttp.Client{
		ReadTimeout:                   timeout,
		WriteTimeout:                  timeout,
		MaxIdleConnDuration:           maxIdleConnDuration,
		MaxConnsPerHost:               2048,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		TLSConfig:                     tlsConfig,
	}
	clients[key] = client

	return client, nil
}

// tlsConfig builds the TLS settings for this configuration, or nil to keep fasthttp's defaults:
// verification against the system roots, with the dialled host as the name to verify.
func (k clientKey) tlsConfig() (*tls.Config, error) {
	if !k.insecureSkipTLSVerify && k.caCert == "" && k.serverName == "" {
		return nil, nil
	}

	config := &tls.Config{
		// Opt-in through the insecureSkipTLSVerify field of the probe.
		InsecureSkipVerify: k.insecureSkipTLSVerify, //nolint:gosec
		ServerName:         k.serverName,
	}

	if k.caCert != "" {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(k.caCert)) {
			return nil, fmt.Errorf("caCert contains no valid PEM certificate")
		}
		// RootCAs is what a client verifies the server against; ClientCAs is the server-side
		// counterpart and has no effect here.
		config.RootCAs = roots
	}

	return config, nil
}

type FastHTTPProbeTarget struct {
	insecureSkipTLSVerify bool
	targetPort            int
	successThreshold      int32
	failureThreshold      int32
	successCount          int32
	failureCount          int32
	timeoutSeconds        int32
	targetHost            string
	host                  string
	path                  string
	scheme                string
	method                string
	caCert                string
	httpHeaders           []corev1.HTTPHeader
	codes                 []int32
}

func (h FastHTTPProbeTarget) GetID() string {
	var sb strings.Builder
	sb.WriteString("http#")
	sb.WriteString(h.targetHost)
	sb.WriteString("#")
	sb.WriteString(fmt.Sprintf("%d", h.targetPort))
	sb.WriteString("#")
	sb.WriteString(h.path)
	sb.WriteString("#")
	sb.WriteString(h.host)
	return sb.String()
}

func (h FastHTTPProbeTarget) SetSuccessCount(count int32) Prober {
	h.successCount = count
	return h
}

func (h FastHTTPProbeTarget) SetFailureCount(count int32) Prober {
	h.failureCount = count
	return h
}

func (h FastHTTPProbeTarget) FailureCount() int32 {
	return h.failureCount
}

func (h FastHTTPProbeTarget) SuccessCount() int32 {
	return h.successCount
}

func (h FastHTTPProbeTarget) SuccessThreshold() int32 {
	return h.successThreshold
}

func (h FastHTTPProbeTarget) FailureThreshold() int32 {
	return h.failureThreshold
}

func (h FastHTTPProbeTarget) isTLS() bool {
	return strings.EqualFold(h.scheme, string(corev1.URISchemeHTTPS))
}

// clientKey collects the settings that have to be baked into the client rather than set per
// request. TLS fields are only carried for an HTTPS probe, so plain HTTP targets keep sharing one
// client per timeout.
func (h FastHTTPProbeTarget) clientKey() clientKey {
	key := clientKey{timeoutSeconds: h.timeoutSeconds}
	if !h.isTLS() {
		return key
	}

	key.insecureSkipTLSVerify = h.insecureSkipTLSVerify
	key.caCert = h.caCert
	// A probe dials the pod by its IP, so there is no name for the certificate to be verified
	// against unless the probe supplies a host. Reusing it as the SNI name is what makes caCert
	// usable without also disabling verification.
	key.serverName = h.host

	return key
}

// url builds the request URL. The client runs with DisablePathNormalizing so that a probe hits
// exactly the configured path, which also means nothing collapses a doubled slash on the way out:
// the path has to be joined, not concatenated onto a separator of our own.
func (h FastHTTPProbeTarget) url() string {
	scheme := strings.ToLower(h.scheme)
	if scheme == "" {
		scheme = strings.ToLower(string(corev1.URISchemeHTTP))
	}

	// The CRD requires a leading slash, but an object stored before that validation existed can
	// still be missing one, and validation does not run on read.
	path := h.path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// JoinHostPort brackets an IPv6 literal, which a plain host:port concatenation would not.
	return scheme + "://" + net.JoinHostPort(h.targetHost, strconv.Itoa(h.targetPort)) + path
}

// statusAccepted reports whether a response code counts as a successful probe. An empty code list
// accepts the whole 200-399 range, which is what a Kubernetes HTTP probe treats as healthy.
func (h FastHTTPProbeTarget) statusAccepted(code int) bool {
	if len(h.codes) == 0 {
		return code >= 200 && code < 400
	}
	return slices.Contains(h.codes, int32(code))
}

func (h FastHTTPProbeTarget) expectedStatus() string {
	if len(h.codes) == 0 {
		return "200-399"
	}

	codes := make([]string, 0, len(h.codes))
	for _, code := range h.codes {
		codes = append(codes, strconv.Itoa(int(code)))
	}

	return strings.Join(codes, ", ")
}

func (h FastHTTPProbeTarget) PerformCheck() error {
	client, err := clientFor(h.clientKey())
	if err != nil {
		return err
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(h.url())
	req.Header.SetMethod(h.method)

	if h.host != "" {
		// SetRequestURI marks the URI as parsed, after which fasthttp overwrites the Host header
		// with the host from the URI unless UseHostHeader is set. Adding a "Host" header on its own
		// is discarded without a trace.
		req.Header.SetHost(h.host)
		req.UseHostHeader = true
	}
	for i := range h.httpHeaders {
		req.Header.Add(h.httpHeaders[i].Name, h.httpHeaders[i].Value)
	}

	if err := client.Do(req, resp); err != nil {
		return err
	}
	if !h.statusAccepted(resp.StatusCode()) {
		return fmt.Errorf("HTTP bad status code %d, expected %s", resp.StatusCode(), h.expectedStatus())
	}

	return nil
}

func (h FastHTTPProbeTarget) GetPort() int {
	return h.targetPort
}

func (h FastHTTPProbeTarget) GetMode() string {
	return "http"
}
