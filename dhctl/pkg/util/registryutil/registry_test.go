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
	"errors"
	"crypto/x509"
	"crypto/sha256"
	"encoding/pem"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

const testRootCA = `
-----BEGIN CERTIFICATE-----
MIIBjzCCATagAwIBAgIUV/km/wXwIMcG5jDQfunCvkvEw9UwCgYIKoZIzj0EAwIw
FDESMBAGA1UEAwwJdGVzdC1yb290MB4XDTI2MDQwNzEzMDQwM1oXDTM2MDQwNDEz
MDQwM1owFDESMBAGA1UEAwwJdGVzdC1yb290MFkwEwYHKoZIzj0CAQYIKoZIzj0D
AQcDQgAEQElWD991NP9xuFaGgX4AkpBfArT+mbN1JJJ6RziAA+/Iq0MKO5UR0xXJ
x0MTD5AGsZ7w64roEoHOK9OwQkxHEqNmMGQwHQYDVR0OBBYEFMPONG/rtHYEvzLs
Vqek0aV3UZBQMB8GA1UdIwQYMBaAFMPONG/rtHYEvzLsVqek0aV3UZBQMBIGA1Ud
EwEB/wQIMAYBAf8CAQEwDgYDVR0PAQH/BAQDAgEGMAoGCCqGSM49BAMCA0cAMEQC
IAsHvdzwmJ2iQbRmblVebWHwRWS+6OwK5sThiiaQykqTAiAMJ/Orkt/ODRVLN8K6
ybQGqcGjzy6jkT4Id0CtXtibag==
-----END CERTIFICATE-----
`

const testIntermediateCA = `
-----BEGIN CERTIFICATE-----
MIIBmTCCAT6gAwIBAgIUYMStnQvnFeu/5Io+bKgAhCDFdF8wCgYIKoZIzj0EAwIw
FDESMBAGA1UEAwwJdGVzdC1yb290MB4XDTI2MDQwNzEzMDQwM1oXDTM2MDQwNDEz
MDQwM1owHDEaMBgGA1UEAwwRdGVzdC1pbnRlcm1lZGlhdGUwWTATBgcqhkjOPQIB
BggqhkjOPQMBBwNCAATBb8651k9p0jBJsitSYTuUe7hAI6XcTEACH8HzE0g1zj7z
xqJCaIhEafBTJRWev/UD4xh3w5ob0UXI7EBR49LGo2YwZDASBgNVHRMBAf8ECDAG
AQH/AgEAMA4GA1UdDwEB/wQEAwIBBjAdBgNVHQ4EFgQUX/05+tB13tgHdMYnSZD/
4q27cGswHwYDVR0jBBgwFoAUw840b+u0dgS/MuxWp6TRpXdRkFAwCgYIKoZIzj0E
AwIDSQAwRgIhAJUjIJ1RptYPRXOwHKlgp3pzFuEvm27U8HTFRQLb38TqAiEA/VvL
ybKXVBFaqtGvrR3o9c+eo57A4wDYwFrUHVJaBo8=
-----END CERTIFICATE-----
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

func TestNewRegistryClient_WithMultipleCAs(t *testing.T) {
	client, err := NewRegistryClient("HTTPS", testRootCA+"\n"+testIntermediateCA)
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.NotNil(t, transport.TLSClientConfig.RootCAs)

	rootCert := parsePEMCertificate(t, testRootCA)
	intermediateCert := parsePEMCertificate(t, testIntermediateCA)

	require.True(t, certPoolContains(transport.TLSClientConfig.RootCAs, rootCert), "root CA should be added to cert pool")
	require.True(t, certPoolContains(transport.TLSClientConfig.RootCAs, intermediateCert), "intermediate CA should be added to cert pool")
}

func TestNewRegistryClient_WithCA(t *testing.T) {
	client, err := NewRegistryClient("HTTPS", testRootCA)
	require.NoError(t, err)
	require.NotNil(t, client.Transport)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.NotNil(t, transport.TLSClientConfig.RootCAs)
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestNewRegistryClient_WithCAAndSystemPoolError(t *testing.T) {
	originalSystemCertPool := systemCertPool
	systemCertPool = func() (*x509.CertPool, error) {
		return nil, errors.New("boom")
	}
	t.Cleanup(func() {
		systemCertPool = originalSystemCertPool
	})

	client, err := NewRegistryClient("HTTPS", testRootCA)
	require.NoError(t, err)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.NotNil(t, transport.TLSClientConfig.RootCAs)
	require.True(t, certPoolContains(transport.TLSClientConfig.RootCAs, parsePEMCertificate(t, testRootCA)))
}

func parsePEMCertificate(t *testing.T, certPEM string) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return cert
}

func certPoolContains(pool *x509.CertPool, cert *x509.Certificate) bool {
	haveSum := reflect.ValueOf(pool).Elem().FieldByName("haveSum")
	sum := sha256.Sum224(cert.Raw)
	return haveSum.MapIndex(reflect.ValueOf(sum)).IsValid()
}
