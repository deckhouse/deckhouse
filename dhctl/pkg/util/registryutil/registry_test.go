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
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const dockerCA = `
-----BEGIN CERTIFICATE-----
MIIDmTCCAx+gAwIBAgISBRFWf+VQa6t1mLBvtv63MpqaMAoGCCqGSM49BAMDMDIx
CzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MQswCQYDVQQDEwJF
ODAeFw0yNjAzMTIwMjAxMjRaFw0yNjA2MTAwMjAxMjNaMBkxFzAVBgNVBAMTDmF1
dGguZG9ja2VyLmlvMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAESagTRweeO9ow
U7FO4pLa3tH7rjZVq4XEZhQdMegc3fl50lFTbKNa2Gq+pmUWnFhCM7RDQUW0kSSh
GW/GawvJ46OCAiwwggIoMA4GA1UdDwEB/wQEAwIHgDATBgNVHSUEDDAKBggrBgEF
BQcDATAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBSUOW+i9EzHGJw7U0A2rR2mil3X
pjAfBgNVHSMEGDAWgBSPDROi9i5+0VBsMxg4XVmOI3KRyjAyBggrBgEFBQcBAQQm
MCQwIgYIKwYBBQUHMAKGFmh0dHA6Ly9lOC5pLmxlbmNyLm9yZy8wKwYDVR0RBCQw
IoIQKi5hdXRoLmRvY2tlci5pb4IOYXV0aC5kb2NrZXIuaW8wEwYDVR0gBAwwCjAI
BgZngQwBAgEwLQYDVR0fBCYwJDAioCCgHoYcaHR0cDovL2U4LmMubGVuY3Iub3Jn
LzI3LmNybDCCAQwGCisGAQQB1nkCBAIEgf0EgfoA+AB3AJaXZL9VWJet90OHaDcI
Qnfp8DrV9qTzNm5GpD8PyqnGAAABnN/8hjYAAAQDAEgwRgIhAPInk9lwP+1nGQ/U
umEeEgUYC5I1HgLUYdnWuyXwr8TUAiEAnxBGiUf4ceTSfJAP93H2O2LsLw2hp2v1
qpyhpuK0ly0AfQDjI43yjaKI4KrgrPD6kMmF8La/9dKlJ7AB/BxEWMS26AAAAZzf
/I3rAAgAAAUANU5lnQQDAEYwRAIgTtvbfhOggi4maccZoq3EQEGYBnXxAQY+Jh2h
1p061SMCIFCjsXy0Sd7DIVCk808DxRdpQuSRA32PRXspicr2udHZMAoGCCqGSM49
BAMDA2gAMGUCMDezs6xgETA8aONpBMezoCvUsOnJPMoPPkRsEe1AFXNX+Q6+UqK6
hc2cOifg6AHgzQIxAKAts5ehw2GieCxkL3B5pDidXNxtVmh1LwoUh7EqZKxHaSVD
CRl8TSg922cXTLVt8Q==
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

func TestNewRegistryClient_WithCA(t *testing.T) {
	client, err := NewRegistryClient("HTTPS", dockerCA)
	require.NoError(t, err)
	require.NotNil(t, client.Transport)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	require.NotNil(t, transport.TLSClientConfig.RootCAs)
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
}
