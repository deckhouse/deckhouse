// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registryutil

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const testCA = `-----BEGIN CERTIFICATE-----
MIIBVDCB+6ADAgECAgEBMAoGCCqGSM49BAMCMBIxEDAOBgNVBAMTB3Rlc3QtY2Ew
HhcNMjYwMTAxMDAwMDAwWhcNMzYwMTAxMDAwMDAwWjASMRAwDgYDVQQDEwd0ZXN0
LWNhMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEKKaMMINPRKyWO9Tu0BQGPMBk
1lKs0EK0Mfo703X/ECvQnosTBbtytNeBSRWv5hxcBpBBPh2bW/PUDgxbIgRvlqNC
MEAwDgYDVR0PAQH/BAQDAgIEMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFPra
qZ7RKqtMutQAOq7uGZuVAnYOMAoGCCqGSM49BAMCA0gAMEUCIQCVrx1CY1SQTljc
6JRqfqWzLJ1mBg5W6AVtEOBqqwtdYwIgQ9GeRIkVThfe4Y2oaDPVhGY+N+JihtTq
/N35+Z0JuPg=
-----END CERTIFICATE-----
`

const testServerCert = `-----BEGIN CERTIFICATE-----
MIIBUzCB+qADAgECAgECMAoGCCqGSM49BAMCMBIxEDAOBgNVBAMTB3Rlc3QtY2Ew
HhcNMjYwMTAxMDAwMDAwWhcNMzYwMTAxMDAwMDAwWjAUMRIwEAYDVQQDEwlsb2Nh
bGhvc3QwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASmfnFyoh4lvwlkAf2NLrSN
UdQZusn/5XZglBlGDphg3NiG8rPfLn8OgaCLSnuSQ+RFxc5fqO9z9HlyVRGMJ/of
oz8wPTAfBgNVHSMEGDAWgBT62qme0SqrTLrUADqu7hmblQJ2DjAaBgNVHREEEzAR
gglsb2NhbGhvc3SHBH8AAAEwCgYIKoZIzj0EAwIDSAAwRQIgMloj6VV2db+xNiI7
ZWASqxSwgg9Ig1V4zdjgMOE+x94CIQDVA8rZY0nm56+8/0a8/Or1TyVnVy9ahWlh
K5PCAqGhKA==
-----END CERTIFICATE-----
`

const testServerKey = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIE5BUq71quQqgKAjiG6dx6gEU3eeippNpMaWj6GTM2pLoAoGCCqGSM49
AwEHoUQDQgAEpn5xcqIeJb8JZAH9jS60jVHUGbrJ/+V2YJQZRg6YYNzYhvKz3y5/
DoGgi0p7kkPkRcXOX6jvc/R5clURjCf6Hw==
-----END EC PRIVATE KEY-----
`

const testRootCA = `-----BEGIN CERTIFICATE-----
MIIBWTCB/6ADAgECAgEKMAoGCCqGSM49BAMCMBQxEjAQBgNVBAMTCXRlc3Qtcm9v
dDAeFw0yNjAxMDEwMDAwMDBaFw0zNjAxMDEwMDAwMDBaMBQxEjAQBgNVBAMTCXRl
c3Qtcm9vdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFDXyowa6m8lk7rJoPTT
u3v5FHVizXs5qh1QCgoefS3WwKny9zWXJ0Y4WxZS5Ay4ASILhEiCCEOlLRtyi3Ju
ABajQjBAMA4GA1UdDwEB/wQEAwICBDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQW
BBQGopT6D7mjK6O9EKWxPPnEbR7zrjAKBggqhkjOPQQDAgNJADBGAiEAw1Fz8Dkh
lfZWNvgIJ/EZE9jFFls7twS783KluszFlagCIQC7M3qeoHZlvHkMY0/h4ZvNULUR
v1S92d6sWpSParjKWQ==
-----END CERTIFICATE-----
`

const testIntermediateCA = `-----BEGIN CERTIFICATE-----
MIIBgjCCASigAwIBAgIBCzAKBggqhkjOPQQDAjAUMRIwEAYDVQQDEwl0ZXN0LXJv
b3QwHhcNMjYwMTAxMDAwMDAwWhcNMzYwMTAxMDAwMDAwWjAcMRowGAYDVQQDExF0
ZXN0LWludGVybWVkaWF0ZTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABNF/aY4a
Fil/MJFLD5CGixCn6Y/tjavxFmCx+vIb4GJFTIklNpeGWCFy+BA1ox7qx6PlESMj
CE5Sx1Y9byiRkyqjYzBhMA4GA1UdDwEB/wQEAwICBDAPBgNVHRMBAf8EBTADAQH/
MB0GA1UdDgQWBBSLuZDJIT0/tINa923wXemSSJbKMTAfBgNVHSMEGDAWgBQGopT6
D7mjK6O9EKWxPPnEbR7zrjAKBggqhkjOPQQDAgNIADBFAiEAh+hDC6+r0HKifjsW
ledEU/5QkZJeTdx6fSIepf8uyuwCIAr6ismHQhtvEnaxGAW329+2+5gCJkTuZk8N
Y3stE7TX
-----END CERTIFICATE-----
`

const testChainServerCert = `-----BEGIN CERTIFICATE-----
MIIBXjCCAQSgAwIBAgIBDDAKBggqhkjOPQQDAjAcMRowGAYDVQQDExF0ZXN0LWlu
dGVybWVkaWF0ZTAeFw0yNjAxMDEwMDAwMDBaFw0zNjAxMDEwMDAwMDBaMBQxEjAQ
BgNVBAMTCWxvY2FsaG9zdDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABF0F3/8r
j7EJ4SfyNWWp/nO//Yy3Ipaqq18SZK7+r2WsEH6yZFFfLdy18kQfTHZG52zEOWff
hGdWZ2g+rbhelcejPzA9MB8GA1UdIwQYMBaAFIu5kMkhPT+0g1r3bfBd6ZJIlsox
MBoGA1UdEQQTMBGCCWxvY2FsaG9zdIcEfwAAATAKBggqhkjOPQQDAgNIADBFAiEA
xg1/EyfRZ6T0feB37Hp13CishrIYElzOQm8d6P7tXrQCIA9+kJHC62FN1c2rvnw/
3wDlcpkhOUqyR1ghOljQvQk3
-----END CERTIFICATE-----
`

const testChainServerKey = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEILgUcCFxegMEqc2vJjqwjVMbktc7ohtXMQEn/PGJhbxkoAoGCCqGSM49
AwEHoUQDQgAEXQXf/yuPsQnhJ/I1Zan+c7/9jLcilqqrXxJkrv6vZawQfrJkUV8t
3LXyRB9MdkbnbMQ5Z9+EZ1ZnaD6tuF6Vxw==
-----END EC PRIVATE KEY-----
`

func TestNewRegistryTransport_HTTP(t *testing.T) {
	transport, err := NewRegistryTransport("HTTP", "")
	require.NoError(t, err)
	require.NotNil(t, transport.TLSClientConfig)
	require.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestNewRegistryTransport_InvalidCA(t *testing.T) {
	_, err := NewRegistryTransport("HTTPS", "-----BEGIN CERTIFICATE-----")
	require.EqualError(t, err, "invalid cert in CA PEM")
}

func TestNewRegistryClient_WithCA(t *testing.T) {
	serverTLSCert, err := tls.X509KeyPair([]byte(testServerCert), []byte(testServerKey))
	require.NoError(t, err)

	server := newTestTLSServer(t, serverTLSCert)

	client, err := NewRegistryClient("HTTPS", testCA)
	require.NoError(t, err)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestNewRegistryClient_WithChainCA(t *testing.T) {
	serverTLSCert, err := tls.X509KeyPair([]byte(testChainServerCert), []byte(testChainServerKey))
	require.NoError(t, err)

	server := newTestTLSServer(t, serverTLSCert)

	client, err := NewRegistryClient("HTTPS", testRootCA+testIntermediateCA)
	require.NoError(t, err)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func newTestTLSServer(t *testing.T, cert tls.Certificate) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}
