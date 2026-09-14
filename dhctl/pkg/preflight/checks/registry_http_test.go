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

package checks

import (
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_mocks "github.com/deckhouse/deckhouse/dhctl/pkg/config/registrymocks"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// A registry answered by httptest, so the checks are exercised over real HTTP rather than over a
// mock of the client. The test this replaces reached registry.deckhouse.io for real: it failed
// wherever the network was unavailable, and it passed for the wrong reason.

// serverCA is the httptest certificate as a PEM bundle, which is what an operator puts in the
// registry CA field for a registry signed by their own authority.
func serverCA(t *testing.T, server *httptest.Server) string {
	t.Helper()

	certificate := server.Certificate()
	require.NotNil(t, certificate, "the test server must be a TLS one")

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))
}

// registryServer starts a TLS server that answers /v2/ the way handler says, and returns a
// MetaConfig pointed at it with its certificate trusted — the position an operator is in once
// they have configured the CA of their registry.
func registryServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *config.MetaConfig) {
	t.Helper()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo(strings.TrimPrefix(server.URL, "https://")+"/deckhouse/ee"),
		registry_mocks.WithSchemeHTTPS(),
		registry_mocks.WithCA(serverCA(t, server)),
	)

	return server, &config.MetaConfig{Registry: registryCfg}
}

// withRegistryAPIVersion writes the header a registry API v2 endpoint always carries.
func withRegistryAPIVersion(w http.ResponseWriter) {
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
}

// TestRegistryReachableAgainstARealServer covers the causes that used to share one sentence,
// "authentication failed", with four unrelated others.
func TestRegistryReachableAgainstARealServer(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantErr    string
		wantDetail string
	}{
		{
			name: "the registry allows anonymous access",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusOK)
			},
			wantDetail: "allows anonymous access",
		},
		{
			name: "the registry asks for credentials",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				withRegistryAPIVersion(w)
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantDetail: "asks for credentials",
		},
		{
			// Something is listening, and it is not a registry: a reverse proxy, a load balancer,
			// an error page.
			name: "an error page answers at the address",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("<html>404</html>"))
			},
			wantErr: "HTTP 404, which is not how a registry answers /v2/",
		},
		{
			// A proxy in front of the registry that strips the header.
			name: "no API version header",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			wantErr: "carries no Docker-Distribution-API-Version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, metaConfig := registryServer(t, tt.handler)

			check := RegistryReachableCheck{MetaConfig: metaConfig}
			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, tt.wantDetail)
		})
	}
}

// TestRegistryReachableRefusesAnUnknownCA: the certificate httptest signs with is not in the
// system pool, which is what a private registry looks like before its CA is configured.
func TestRegistryReachableRefusesAnUnknownCA(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		withRegistryAPIVersion(w)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	registryCfg := registry_mocks.ConfigBuilder(
		registry_mocks.WithImagesRepo(strings.TrimPrefix(server.URL, "https://")+"/deckhouse/ee"),
		registry_mocks.WithSchemeHTTPS(),
	)

	check := RegistryReachableCheck{MetaConfig: &config.MetaConfig{Registry: registryCfg}}
	_, err := check.Run(t.Context())

	require.Error(t, err)

	var failure *preflight.Failure
	require.ErrorAs(t, err, &failure)
	assert.Contains(t, failure.Observed, "certificate signed by unknown authority")
	assert.Contains(t, failure.Fix, "registry CA",
		"a private CA is the fix for this, and it is a different fix from every other transport error")
}

// TestRegistryCredentialsAgainstARealServer covers the answers the credentials check has to tell
// apart: the password is wrong, the password is right but grants no pull, and the registry speaks
// Basic only — which the old text answered with "consider enabling bearer token auth", advice
// that sends the operator to change a registry working as designed.
func TestRegistryCredentialsAgainstARealServer(t *testing.T) {
	const user, password = "license-token", "secret"

	basicAuth := func(r *http.Request) bool {
		encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
		return r.Header.Get("Authorization") == "Basic "+encoded
	}

	tests := []struct {
		name       string
		handler    func(server *httptest.Server) http.HandlerFunc
		wantErr    string
		wantDetail string
	}{
		{
			name: "basic auth accepts them",
			handler: func(*httptest.Server) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					withRegistryAPIVersion(w)
					if !basicAuth(r) {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.WriteHeader(http.StatusOK)
				}
			},
			wantDetail: "(basic auth)",
		},
		{
			name: "the token endpoint accepts them",
			handler: func(server *httptest.Server) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					withRegistryAPIVersion(w)
					if strings.HasPrefix(r.URL.Path, "/auth/token") {
						if !basicAuth(r) {
							w.WriteHeader(http.StatusUnauthorized)
							return
						}
						w.WriteHeader(http.StatusOK)
						return
					}
					w.Header().Set("WWW-Authenticate",
						fmt.Sprintf(`Bearer realm="%s/auth/token",service="registry"`, server.URL))
					w.WriteHeader(http.StatusUnauthorized)
				}
			},
			wantDetail: "(bearer token)",
		},
		{
			name: "the token endpoint turns them down",
			handler: func(server *httptest.Server) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					withRegistryAPIVersion(w)
					if strings.HasPrefix(r.URL.Path, "/auth/token") {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.Header().Set("WWW-Authenticate",
						fmt.Sprintf(`Bearer realm="%s/auth/token",service="registry"`, server.URL))
					w.WriteHeader(http.StatusUnauthorized)
				}
			},
			wantErr: "rejected the credentials",
		},
		{
			name: "they authenticate but grant no pull",
			handler: func(server *httptest.Server) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					withRegistryAPIVersion(w)
					if strings.HasPrefix(r.URL.Path, "/auth/token") {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					w.Header().Set("WWW-Authenticate",
						fmt.Sprintf(`Bearer realm="%s/auth/token",service="registry"`, server.URL))
					w.WriteHeader(http.StatusUnauthorized)
				}
			},
			wantErr: "no pull permission",
		},
		{
			// Nexus, Harbor with basic, registry:2 behind htpasswd: there is no bearer realm to
			// find, and the password is simply wrong.
			name: "a basic-only registry with a wrong password",
			handler: func(*httptest.Server) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					withRegistryAPIVersion(w)
					w.Header().Set("WWW-Authenticate", `Basic realm="Sonatype Nexus Repository Manager"`)
					w.WriteHeader(http.StatusUnauthorized)
				}
			},
			wantErr: "uses basic authentication and rejected the credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.handler(server)(w, r)
			}))
			t.Cleanup(server.Close)

			registryCfg := registry_mocks.ConfigBuilder(
				registry_mocks.WithImagesRepo(strings.TrimPrefix(server.URL, "https://")+"/deckhouse/ee"),
				registry_mocks.WithSchemeHTTPS(),
				registry_mocks.WithCA(serverCA(t, server)),
				registry_mocks.WithCredentials(user, password),
			)

			check := RegistryCredentialsCheck{
				MetaConfig:    &config.MetaConfig{Registry: registryCfg},
				InstallConfig: &config.DeckhouseInstaller{Registry: registryCfg, DevBranch: "main"},
			}

			detail, err := check.Run(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, detail, tt.wantDetail)
		})
	}
}
