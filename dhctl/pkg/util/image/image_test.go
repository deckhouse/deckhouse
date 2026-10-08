// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeDockerConfig(t *testing.T) {
	dummyAuth := make(map[string]authEntry)
	dummyAuth["docker.io"] = authEntry{Auth: "dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}
	t.Run("Docker config", func(t *testing.T) {
		cases := []struct {
			title    string
			input    string
			expected *dockerConfig
			wantErr  bool
			err      string
		}{
			{
				title: "Valid config, success",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}}}
				input:    "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				expected: &dockerConfig{Auths: dummyAuth},
				wantErr:  false,
			},
			{
				title:   "Invalid config, decode failure",
				input:   "hello world",
				wantErr: true,
				err:     "decoding base64 dockerconfig: illegal base64 data at input byte 5",
			},
			{
				title:   "Invalid config, parsing failure",
				input:   "aGVsbG8gd29ybGQK",
				wantErr: true,
				err:     "unmarshaling dockerconfig JSON: invalid character 'h' looking for beginning of value",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				dc, err := DecodeDockerConfig(c.input)
				if c.wantErr {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, c.expected, dc)
				}
			})
		}
	})
}

func TestRegistryConfigFromDockerConfig(t *testing.T) {
	t.Run("Registry config from dockerconfig", func(t *testing.T) {
		cases := []struct {
			title         string
			encodedConfig string
			registry      string
			scheme        string
			expected      *RegistryConfig
			wantErr       bool
			err           string
		}{
			{
				title: "Valid config, success",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				registry:      "docker.io",
				scheme:        "HTTPS",
				expected:      &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test"},
				wantErr:       false,
			},
			{
				title: "Valid config, registry with path, success",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				registry:      "docker.io/any/path",
				scheme:        "HTTPS",
				expected:      &RegistryConfig{registry: "docker.io/any/path", scheme: "HTTPS", username: "test", password: "test-test-test"},
				wantErr:       false,
			},
			{
				title: "Valid config, username/password instead of auth, success",
				// {"auths":{"docker.io":{"username":"test","password":"test-test-test"}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsidXNlcm5hbWUiOiJ0ZXN0IiwicGFzc3dvcmQiOiJ0ZXN0LXRlc3QtdGVzdCJ9fX0=",
				registry:      "docker.io",
				scheme:        "HTTPS",
				expected:      &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test"},
				wantErr:       false,
			},
			{
				title: "Invalid scheme, failure",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				registry:      "docker.io",
				scheme:        "SCHEME",
				wantErr:       true,
				err:           "scheme must be HTTP or HTTPS",
			},
			{
				title: "Invalid config, lack registry, failure",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				registry:      "registry.io",
				scheme:        "HTTPS",
				wantErr:       true,
				err:           "docker config doesn't contain registry.io registry credentials",
			},
			{
				title: "Invalid auth, failure",
				// {"auths":{"docker.io":{"auth":"dGVzdDp0ZXN0LXRlc3Q"}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkRHAwWlhOMExYUmxjM1EifX19",
				registry:      "docker.io",
				scheme:        "HTTPS",
				wantErr:       true,
				err:           "decoding auth field: illegal base64 data at input byte 16",
			},
			{
				title: "Invalid auth format, failure",
				// {"auths":{"docker.io":{"auth":"dGVzdC10ZXN0LXRlc3QtdGVzdA=="}}}
				encodedConfig: "eyJhdXRocyI6eyJkb2NrZXIuaW8iOnsiYXV0aCI6ImRHVnpkQzEwWlhOMExYUmxjM1F0ZEdWemRBPT0ifX19",
				registry:      "docker.io",
				scheme:        "HTTPS",
				wantErr:       true,
				err:           "invalid auth format, missing ':'",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				dc, err := DecodeDockerConfig(c.encodedConfig)
				require.NoError(t, err)
				rc, err := RegistryConfigFromDockerConfig(dc, c.scheme, c.registry)
				if c.wantErr {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, c.expected, rc)
				}
			})
		}
	})
}

func TestAuthFromRegistryConfig(t *testing.T) {
	t.Run("Get auth from RegistryConfig", func(t *testing.T) {
		cases := []struct {
			title    string
			rc       *RegistryConfig
			registry string
			expected *authn.Basic
			wantErr  bool
			err      string
		}{
			{
				title:    "Valid config and registry, success",
				rc:       &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test"},
				registry: "docker.io",
				expected: &authn.Basic{Username: "test", Password: "test-test-test"},
				wantErr:  false,
			},
			{
				title:    "Valid config, invalid registry, failure",
				rc:       &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test"},
				registry: "<<<{}-99987jhy",
				wantErr:  true,
				err:      "parsing registry URL \"<<<{}-99987jhy\": parse \"https://<<<{}-99987jhy\": invalid character \"{\" in host name",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				auth, err := authFromRegistry(c.rc, c.registry)
				if c.wantErr {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, c.expected, auth)
				}
			})
		}
	})
}

func TestRegistryConfig(t *testing.T) {
	t.Run("RegistryConfig tests", func(t *testing.T) {
		cases := []struct {
			title    string
			registry string
			scheme   string
			username string
			password string
			ca       string
			expected *RegistryConfig
			wantErr  bool
			err      string
		}{
			{
				title:    "Valid, success",
				scheme:   "HTTPS",
				registry: "docker.io",
				username: "test",
				password: "test-test-test",
				ca:       "-----BEGIN CERTIFICATE-----",
				expected: &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test", ca: "-----BEGIN CERTIFICATE-----"},
				wantErr:  false,
			},
			{
				title:    "Valid, no registry",
				scheme:   "HTTPS",
				username: "test",
				password: "test-test-test",
				ca:       "-----BEGIN CERTIFICATE-----",
				expected: &RegistryConfig{scheme: "HTTPS", username: "test", password: "test-test-test", ca: "-----BEGIN CERTIFICATE-----"},
				wantErr:  false,
			},
			{
				title:    "Invalid, no scheme",
				registry: "docker.io",
				username: "test",
				password: "test-test-test",
				ca:       "-----BEGIN CERTIFICATE-----",
				wantErr:  true,
				err:      "scheme must be HTTP or HTTPS",
			},
			{
				title:    "Valid, no creds and ca",
				scheme:   "HTTPS",
				registry: "docker.io",
				expected: &RegistryConfig{registry: "docker.io", scheme: "HTTPS"},
				wantErr:  false,
			},
			{
				title:    "Wrong scheme, failure",
				scheme:   "SCHEME",
				registry: "docker.io",
				username: "test",
				password: "test-test-test",
				ca:       "-----BEGIN CERTIFICATE-----",
				wantErr:  true,
				err:      "scheme must be HTTP or HTTPS",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				rc, err := NewRegistryConfig(c.scheme, c.registry, c.username, c.password, c.ca)
				if c.wantErr {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, c.expected, rc)
					registry := rc.GetRegistry()
					require.Equal(t, registry, c.registry)
					scheme := rc.GetScheme()
					require.Equal(t, scheme, c.scheme)
					username := rc.GetUsername()
					require.Equal(t, username, c.username)
					password := rc.GetPassword()
					require.Equal(t, password, c.password)
					ca := rc.GetCA()
					require.Equal(t, ca, c.ca)
				}
			})
		}
	})

	t.Run("set and get ca", func(t *testing.T) {
		rc := &RegistryConfig{registry: "docker.io", scheme: "HTTPS", username: "test", password: "test-test-test"}
		ca := "-----BEGIN CERTIFICATE-----"
		rc.SetCA(ca)
		require.Equal(t, ca, rc.GetCA())
	})
}

func TestGetRegistries(t *testing.T) {
	dummyAuth := make(map[string]authEntry)
	dummyAuth["docker.io"] = authEntry{Auth: "dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}
	dummyAuth["registry.io"] = authEntry{Auth: "dGVzdDp0ZXN0LXRlc3QtdGVzdA=="}
	dc := &dockerConfig{Auths: dummyAuth}
	registries := dc.GetRegistries()
	require.Contains(t, registries, "docker.io")
	require.Contains(t, registries, "registry.io")
}

func TestDownloadAndUnpackImage(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	dockerCA := `
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

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	t.Run("DownloadAndUnpackImage tests", func(t *testing.T) {
		cases := []struct {
			title       string
			directory   string
			rc          RegistryConfig
			image       string
			prepareFunc func() error
			wantErr     bool
			err         string
		}{
			{
				title:     "Success",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "registry.deckhouse.io"},
				// registry.deckhouse.io/deckhouse/ce/release-channel:v1.75.4
				image:   "registry.deckhouse.io/deckhouse/ce/release-channel@sha256:abd4aac6059e1c4fc456b4ce6a81994d06fb87d321bdcb9dd31a81ed04e206cb",
				wantErr: false,
			},
			{
				title:     "Invalid image reference, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "registry.deckhouse.io"},
				image:     "<<<---$#%",
				wantErr:   true,
				err:       "parsing image reference",
			},
			{
				title:     "Invalid image reference, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "registry.deckhouse.io"},
				image:     "registry.deckhouse.io/deckhouse/ce/release-channel:v0.0.1",
				wantErr:   true,
				err:       "pulling image",
			},
			// should be fixed later
			{
				title:     "Wrong CA, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "registry.deckhouse.io", ca: "-----BEGIN CERTIFICATE-----"},
				image:     "registry.deckhouse.io/deckhouse/ce/release-channel@sha256:abd4aac6059e1c4fc456b4ce6a81994d06fb87d321bdcb9dd31a81ed04e206bc",
				wantErr:   true,
				err:       "invalid cert in CA PEM",
			},
			{
				title:     "With docker ca, success",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "registry.deckhouse.io", ca: dockerCA},
				image:     "registry.deckhouse.io/deckhouse/ce/release-channel:v1.75.4",
				prepareFunc: func() error {
					return os.RemoveAll(filepath.Join(testDir, "usr"))
				},
				wantErr: false,
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				ctx := t.Context()
				if c.prepareFunc != nil {
					err = c.prepareFunc()
					require.NoError(t, err)
				}

				err := DownloadAndUnpackImage(ctx, c.image, c.directory, c.rc, false)
				if !c.wantErr {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}

			})
		}
	})
}

// The registry stream checks a layer's digest only at EOF, and tar stops reading at its end
// marker. A layer altered after it (here the gzip size trailer) must still be refused.
func TestDownloadAndUnpackImageRejectsTamperedLayer(t *testing.T) {
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "payload", Mode: 0o644, Size: 5, Typeflag: tar.TypeReg}))
	_, err := tw.Write([]byte("hello"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, err = gw.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	blob := gzBuf.Bytes()

	layer := static.NewLayer(blob, types.DockerLayer)
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)
	layerDigest, err := layer.Digest()
	require.NoError(t, err)
	imgDigest, err := img.Digest()
	require.NoError(t, err)

	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)-1] ^= 0xff

	reg := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blobs/"+layerDigest.String()) {
			w.Header().Set("Docker-Content-Digest", layerDigest.String())
			_, err := w.Write(tampered)
			assert.NoError(t, err)
			return
		}
		reg.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")

	tag, err := name.NewTag(host+"/test/img:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, img))

	err = DownloadAndUnpackImage(t.Context(), host+"/test/img@"+imgDigest.String(), t.TempDir(), RegistryConfig{scheme: "HTTP", registry: host}, false)
	require.ErrorContains(t, err, "checksum")

	root := t.TempDir()
	_, err = EnsureUnpacked(t.Context(), UnpackRequest{
		Root:   root,
		Name:   "candi",
		Digest: imgDigest.String(),
		Registry: func(context.Context) (*RegistryConfig, error) {
			return NewRegistryConfig("HTTP", host+"/test/img", "", "", "")
		},
	}, EnsureUnpackedOptions{})
	require.ErrorContains(t, err, "checksum")
	require.Empty(t, unpackRootEntries(t, root), "a tampered image must not become visible")
}

// ggcr forgives HTTP for localhost and RFC1918, so the host must look public.
const testSchemeHost = "nexus.example.com"

// A plain-HTTP registry behind an ingress that answers 443 with a foreign cert.
func TestDownloadAndUnpackImage_Scheme(t *testing.T) {
	for _, tt := range []struct {
		name        string
		scheme      string
		ca          string
		wantPlain   bool
		wantErrLike string
	}{
		{name: "http without CA", scheme: "HTTP", wantPlain: true},
		{name: "http with CA", scheme: "HTTP", ca: testSchemeCA, wantPlain: true},
		{name: "https refuses the untrusted front", scheme: "HTTPS", wantErrLike: "certificate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plainHits, tlsHits := newTLSFrontedRegistry(t)
			destDir := t.TempDir()

			// the stub has no blobs; what matters is which leg was used
			err := DownloadAndUnpackImage(t.Context(), testSchemeHost+"/deckhouse/ee:v1",
				destDir, RegistryConfig{scheme: tt.scheme, registry: testSchemeHost, ca: tt.ca}, false)

			if tt.wantErrLike != "" {
				require.ErrorContains(t, err, tt.wantErrLike)
			}
			require.Zero(t, tlsHits.Load(), "the untrusted TLS front must never serve the download")
			require.Equal(t, tt.wantPlain, plainHits.Load() > 0, "served over plain HTTP")
		})
	}
}

// Serves testSchemeHost from a plain-HTTP server and an untrusted TLS front, and
// points both transports ggcr may pick (remote.DefaultTransport without a CA,
// http.DefaultTransport with one) at them. Do not add t.Parallel() to this package.
func newTLSFrontedRegistry(t *testing.T) (plainHits, tlsHits *atomic.Int64) {
	t.Helper()

	plainHits, tlsHits = &atomic.Int64{}, &atomic.Int64{}
	plain := httptest.NewServer(registryStub(plainHits))
	tlsServer := httptest.NewTLSServer(registryStub(tlsHits))
	t.Cleanup(plain.Close)
	t.Cleanup(tlsServer.Close)

	port := func(rawURL string) string {
		_, p, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://"))
		require.NoError(t, err)
		return p
	}
	plainPort, tlsPort := port(plain.URL), port(tlsServer.URL)

	routing := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			target := plainPort
			if _, p, _ := net.SplitHostPort(addr); p == "443" {
				target = tlsPort
			}
			var d net.Dialer
			return d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", target))
		},
	}

	originalRemote, originalDefault := remote.DefaultTransport, http.DefaultTransport
	remote.DefaultTransport, http.DefaultTransport = routing, routing
	t.Cleanup(func() {
		remote.DefaultTransport, http.DefaultTransport = originalRemote, originalDefault
	})

	return plainHits, tlsHits
}

func registryStub(hits *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
}

const testSchemeCA = `-----BEGIN CERTIFICATE-----
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
