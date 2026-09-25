/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// newProbeTarget builds a target aimed at srv, with the fields a probe always carries.
func newProbeTarget(t *testing.T, srv *httptest.Server, scheme, path string) FastHTTPProbeTarget {
	t.Helper()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(srv.URL, "http://"), "https://"))
	if err != nil {
		t.Fatalf("cannot split %q: %v", srv.URL, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("cannot parse port %q: %v", portStr, err)
	}

	return FastHTTPProbeTarget{
		targetHost:     host,
		targetPort:     port,
		scheme:         scheme,
		method:         http.MethodGet,
		path:           path,
		timeoutSeconds: 2,
	}
}

// recordingServer answers every request with 200 and records what it received.
func recordingServer(t *testing.T, tlsServer bool) (*httptest.Server, *string, *string) {
	t.Helper()

	var requestURI, hostHeader string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURI = r.RequestURI
		hostHeader = r.Host
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewUnstartedServer(handler)
	if tlsServer {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)

	return srv, &requestURI, &hostHeader
}

// TestProbeSendsConfiguredPath covers the defect where the URL was built by concatenating the path
// onto a hardcoded separator: a CRD path of "/test" reached the backend as "//test", and the client
// runs with DisablePathNormalizing so nothing collapsed it on the way out.
func TestProbeSendsConfiguredPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "leading slash is not doubled", path: "/test", want: "/test"},
		{name: "missing leading slash is added", path: "test", want: "/test"},
		{name: "empty path becomes root", path: "", want: "/"},
		{name: "nested path is preserved", path: "/healthz/ready", want: "/healthz/ready"},
		{name: "query string is preserved", path: "/healthz?full=1", want: "/healthz?full=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, requestURI, _ := recordingServer(t, false)

			target := newProbeTarget(t, srv, "HTTP", tt.path)
			if err := target.PerformCheck(); err != nil {
				t.Fatalf("PerformCheck() = %v, want nil", err)
			}
			if *requestURI != tt.want {
				t.Errorf("server received %q, want %q", *requestURI, tt.want)
			}
		})
	}
}

// TestProbeSendsHostHeader covers the Host override, which was added as an ordinary header and so
// was overwritten by the host from the URI before the request went out.
func TestProbeSendsHostHeader(t *testing.T) {
	srv, _, hostHeader := recordingServer(t, false)

	target := newProbeTarget(t, srv, "HTTP", "/")
	target.host = "backend.example.com"
	if err := target.PerformCheck(); err != nil {
		t.Fatalf("PerformCheck() = %v, want nil", err)
	}
	if *hostHeader != "backend.example.com" {
		t.Errorf("server saw Host %q, want %q", *hostHeader, "backend.example.com")
	}
}

// TestProbeAcceptedStatusCodes pins the contract of the code field: an empty list accepts the whole
// 200-399 range rather than only 200, and a populated list accepts exactly what it names.
func TestProbeAcceptedStatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		codes   []int32
		status  int
		wantErr bool
	}{
		{name: "default accepts 200", status: http.StatusOK},
		{name: "default accepts 204", status: http.StatusNoContent},
		{name: "default accepts 301", status: http.StatusMovedPermanently},
		{name: "default accepts 399", status: 399},
		{name: "default rejects 400", status: http.StatusBadRequest, wantErr: true},
		{name: "default rejects 503", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "explicit list accepts a named code", codes: []int32{201, 202}, status: http.StatusAccepted},
		{name: "explicit list rejects an unnamed code", codes: []int32{201, 202}, status: http.StatusOK, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			t.Cleanup(srv.Close)

			target := newProbeTarget(t, srv, "HTTP", "/")
			target.codes = tt.codes

			err := target.PerformCheck()
			if tt.wantErr && err == nil {
				t.Fatalf("PerformCheck() = nil, want an error for status %d", tt.status)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("PerformCheck() = %v, want nil for status %d", err, tt.status)
			}
		})
	}
}

// certPEM re-encodes the server's certificate so it can be fed back as the probe's caCert.
func certPEM(t *testing.T, srv *httptest.Server) string {
	t.Helper()

	cert := srv.Certificate()
	if cert == nil {
		t.Fatal("test server has no certificate")
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

// TestProbeHTTPSVerification covers both halves of the broken TLS path: the settings were gated on
// the request method rather than the scheme, so the branch never ran, and the CRD fields never
// reached the probe target in the first place. Either one on its own left every HTTPS probe
// verifying against the system roots.
func TestProbeHTTPSVerification(t *testing.T) {
	t.Run("untrusted certificate fails", func(t *testing.T) {
		srv, _, _ := recordingServer(t, true)

		target := newProbeTarget(t, srv, "HTTPS", "/")
		if err := target.PerformCheck(); err == nil {
			t.Fatal("PerformCheck() = nil, want a certificate verification error")
		}
	})

	t.Run("insecureSkipTLSVerify accepts an untrusted certificate", func(t *testing.T) {
		srv, requestURI, _ := recordingServer(t, true)

		target := newProbeTarget(t, srv, "HTTPS", "/test")
		target.insecureSkipTLSVerify = true
		if err := target.PerformCheck(); err != nil {
			t.Fatalf("PerformCheck() = %v, want nil", err)
		}
		if *requestURI != "/test" {
			t.Errorf("server received %q, want %q", *requestURI, "/test")
		}
	})

	t.Run("caCert verifies the server certificate", func(t *testing.T) {
		srv, _, _ := recordingServer(t, true)

		target := newProbeTarget(t, srv, "HTTPS", "/")
		target.caCert = certPEM(t, srv)
		if err := target.PerformCheck(); err != nil {
			t.Fatalf("PerformCheck() = %v, want nil", err)
		}
	})

	t.Run("malformed caCert is reported as a configuration error", func(t *testing.T) {
		srv, _, _ := recordingServer(t, true)

		target := newProbeTarget(t, srv, "HTTPS", "/")
		target.caCert = "not a certificate"

		err := target.PerformCheck()
		if err == nil {
			t.Fatal("PerformCheck() = nil, want an error")
		}
		if !strings.Contains(err.Error(), "caCert") {
			t.Errorf("PerformCheck() = %v, want an error naming caCert", err)
		}
	})
}

// TestClientForSeparatesConfigurations covers the reason the settings above take effect at all.
// fasthttp.Client copies timeouts and TLS settings into a per-host client on first use, so probes
// that differ in any of them must not share a client: the previous code mutated one pooled client
// per request, which the library silently ignored after the first dial.
func TestClientForSeparatesConfigurations(t *testing.T) {
	base := clientKey{timeoutSeconds: 2}

	same, err := clientFor(base)
	if err != nil {
		t.Fatalf("clientFor() = %v", err)
	}
	again, err := clientFor(base)
	if err != nil {
		t.Fatalf("clientFor() = %v", err)
	}
	if same != again {
		t.Error("clientFor() returned different clients for the same configuration, losing connection reuse")
	}

	others := []clientKey{
		{timeoutSeconds: 5},
		{timeoutSeconds: 2, insecureSkipTLSVerify: true},
		{timeoutSeconds: 2, serverName: "backend.example.com"},
	}
	for _, key := range others {
		other, err := clientFor(key)
		if err != nil {
			t.Fatalf("clientFor(%+v) = %v", key, err)
		}
		if other == same {
			t.Errorf("clientFor(%+v) shares a client with %+v", key, base)
		}
	}
}

// TestTLSConfigUsesRootCAs pins the field a client verifies against. The original code appended the
// CA to ClientCAs, the server-side counterpart, which has no effect on an outgoing connection.
func TestTLSConfigUsesRootCAs(t *testing.T) {
	srv, _, _ := recordingServer(t, true)

	key := clientKey{timeoutSeconds: 2, caCert: certPEM(t, srv)}
	config, err := key.tlsConfig()
	if err != nil {
		t.Fatalf("tlsConfig() = %v", err)
	}
	if config.RootCAs == nil {
		t.Fatal("tlsConfig() left RootCAs nil, so caCert would not be trusted")
	}
	if config.ClientCAs != nil {
		t.Error("tlsConfig() set ClientCAs, which has no effect on a client connection")
	}

	// The pool must actually contain the certificate, not merely be non-nil.
	if _, err := srv.Certificate().Verify(x509.VerifyOptions{Roots: config.RootCAs}); err != nil {
		t.Errorf("server certificate does not verify against the built pool: %v", err)
	}
}

// TestTLSConfigDefaultsToSystemRoots keeps a plain HTTP probe, and an HTTPS probe with no TLS
// options, on fasthttp's own defaults rather than a half-built config.
func TestTLSConfigDefaultsToSystemRoots(t *testing.T) {
	config, err := clientKey{timeoutSeconds: 1}.tlsConfig()
	if err != nil {
		t.Fatalf("tlsConfig() = %v", err)
	}
	if config != nil {
		t.Errorf("tlsConfig() = %+v, want nil", config)
	}
}

// TestClientKeyOmitsTLSForPlainHTTP keeps TLS fields out of the key for an HTTP probe, so plain
// probes sharing a timeout keep sharing one client.
func TestClientKeyOmitsTLSForPlainHTTP(t *testing.T) {
	target := FastHTTPProbeTarget{
		scheme:                "HTTP",
		timeoutSeconds:        2,
		insecureSkipTLSVerify: true,
		caCert:                "ignored",
		host:                  "backend.example.com",
	}

	want := clientKey{timeoutSeconds: 2}
	if got := target.clientKey(); got != want {
		t.Errorf("clientKey() = %+v, want %+v", got, want)
	}

	target.scheme = "HTTPS"
	wantTLS := clientKey{
		timeoutSeconds:        2,
		insecureSkipTLSVerify: true,
		caCert:                "ignored",
		serverName:            "backend.example.com",
	}
	if got := target.clientKey(); got != wantTLS {
		t.Errorf("clientKey() = %+v, want %+v", got, wantTLS)
	}
}

// TestProbeURLBracketsIPv6 covers host:port joining for an IPv6 pod address, which a plain
// concatenation would render unparseable.
func TestProbeURLBracketsIPv6(t *testing.T) {
	target := FastHTTPProbeTarget{
		targetHost: "fd00::1",
		targetPort: 8090,
		scheme:     "HTTP",
		path:       "/test",
	}

	want := "http://[fd00::1]:8090/test"
	if got := target.url(); got != want {
		t.Errorf("url() = %q, want %q", got, want)
	}
}
