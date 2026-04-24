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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
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
				err:           "docker config doesn't contains registry.io registry credentials",
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

func TestHashFileSHA256(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	testFile, err := os.CreateTemp(testDir, "testfile")
	require.NoError(t, err)
	testFile.WriteString("Hello world")

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})
	cmd := exec.Command("sha256sum", testFile.Name())
	res, err := cmd.Output()
	require.NoError(t, err)
	parts := strings.Split(string(res), " ")
	shaFromCmd := (parts[0])

	t.Run("Equality of SHA256", func(t *testing.T) {
		shaFromFunc, err := hashFileSHA256(testFile.Name())
		require.NoError(t, err)
		require.Equal(t, shaFromFunc, shaFromCmd)
	})

	t.Run("Non-existent file", func(t *testing.T) {
		_, err := hashFileSHA256("/path/to/nowhere")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no such file")
	})
}

func TestGetHash(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	t.Run("getHash tests", func(t *testing.T) {
		cases := []struct {
			title         string
			directory     string
			content       string
			expectedKey   string
			expectedValue string
			wantErr       bool
			err           string
		}{
			{
				title:         "Success",
				directory:     testDir,
				content:       "{\"some\":\"456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c\"}",
				expectedKey:   "some",
				expectedValue: "456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c",
				wantErr:       false,
			},
			{
				title:         "No entries, empty return, success",
				directory:     testDir,
				content:       "{\"some\":\"456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c\"}",
				expectedKey:   "nonexistant",
				expectedValue: "",
				wantErr:       false,
			},
			{
				title:     "Invalid JSON, failure",
				directory: testDir,
				content:   "Just a string, not JSON",
				wantErr:   true,
				err:       "unmarshalling json: invalid character 'J' looking for beginning of value",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				testFile, err := os.Create(filepath.Join(c.directory, "images_hashs.json"))
				require.NoError(t, err)
				testFile.WriteString(c.content)
				hash, err := getHash(c.expectedKey, c.directory)
				if !c.wantErr {
					require.NoError(t, err)
					require.Equal(t, c.expectedValue, hash)
				} else {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				}

			})
		}
	})

	t.Run("non-existant images_hashs.json", func(t *testing.T) {
		newTestDir := filepath.Join(os.TempDir(), "dhctltests2")
		err := os.MkdirAll(testDir, 0755)
		require.NoError(t, err)

		t.Cleanup(func() {
			os.RemoveAll(newTestDir)
		})
		_, err = getHash("some", newTestDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot open file")
	})

}

func TestSaveHash(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	testDir2 := filepath.Join(os.TempDir(), "dhctltests2")
	err = os.MkdirAll(testDir2, 0755)
	require.NoError(t, err)

	testDir3 := filepath.Join(os.TempDir(), "dhctltests3")
	err = os.MkdirAll(testDir3, 0755)
	require.NoError(t, err)

	testFile, err := os.Create(filepath.Join(testDir, "images_hashs.json"))
	require.NoError(t, err)
	testFile.WriteString("{\"some\":\"456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c\"}")

	testFile3, err := os.Create(filepath.Join(testDir3, "images_hashs.json"))
	require.NoError(t, err)
	testFile3.WriteString("Just invalid JSON")

	t.Cleanup(func() {
		os.RemoveAll(testDir)
		os.RemoveAll(testDir2)
		os.RemoveAll(testDir3)
	})

	t.Run("saveHash tests", func(t *testing.T) {
		cases := []struct {
			title     string
			directory string
			key       string
			value     string
			wantErr   bool
			err       string
		}{
			{
				title:     "Same value, success",
				directory: testDir,
				key:       "some",
				value:     "456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c",
				wantErr:   false,
			},
			{
				title:     "Another value, success",
				directory: testDir,
				key:       "some",
				value:     "f8815c340a1ed5b7ff06c14d99723e9d675236ac86b426fcfd7bd4c1df18b050",
				wantErr:   false,
			},
			{
				title:     "images_hashs.json doesn't exists, success",
				directory: testDir2,
				key:       "some",
				value:     "456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c",
				wantErr:   false,
			},
			{
				title:     "Invalid JSON",
				directory: testDir3,
				key:       "some",
				value:     "456d9c3120dd76b1b6ab00be58ee665abf792bcdf1da49b713246d61c261f05c",
				wantErr:   true,
				err:       "unmarshalling json: invalid character 'J' looking for beginning of value",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {

				err = saveHash(c.key, c.value, c.directory)
				if !c.wantErr {
					require.NoError(t, err)
					hash, err := getHash(c.key, c.directory)
					require.NoError(t, err)
					require.Equal(t, c.value, hash)

					// put another key, the old one must exists after
					err = saveHash("aaa", "bbbbbbbb", c.directory)
					require.NoError(t, err)
					hash, err = getHash(c.key, c.directory)
					require.NoError(t, err)
					require.Equal(t, c.value, hash)
				} else {
					require.Error(t, err)
					if c.err != "" {
						require.Equal(t, c.err, err.Error())
					}
				}

			})
		}
	})
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
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				// docker.io/library/nginx:stable-alpine
				image:   "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e",
				wantErr: false,
			},
			{
				title:     "Cache hit, success",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				// docker.io/library/nginx:stable-alpine
				image: "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e",
				prepareFunc: func() error {
					return os.RemoveAll(filepath.Join(testDir, "usr"))
				},
				wantErr: false,
			},
			{
				title:     "Invalid image reference, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				image:     "<<<---$#%",
				wantErr:   true,
				err:       "parsing image reference",
			},
			{
				title:     "Invalid image reference, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				// docker.io/library/nginx:stable-alpine
				image:   "docker.io/library/nginx:notatag100500",
				wantErr: true,
				err:     "getting manifest descriptor for",
			},
			{
				title:     "Wrong CA, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io", ca: "-----BEGIN CERTIFICATE-----"},
				// docker.io/library/nginx:stable-alpine
				image:   "docker.io/library/nginx:latest",
				wantErr: true,
				err:     "invalid cert in CA PEM",
			},
			{
				title:     "With docker ca, success",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io", ca: dockerCA},
				// docker.io/library/nginx:stable-alpine
				image: "docker.io/library/nginx:latest",
				prepareFunc: func() error {
					return os.RemoveAll(filepath.Join(testDir, "usr"))
				},
				wantErr: false,
			},
			{
				title:     "Cannot pull image, failure",
				directory: testDir,
				rc:        RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				// docker.io/library/nginx:stable-alpine
				image: "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e",
				prepareFunc: func() error {
					if err = os.Remove(filepath.Join(testDir, "images_hashs.json")); err != nil {
						return err
					}
					f, err := os.Create(filepath.Join(testDir, "images_hashs.json"))
					if err != nil {
						return err
					}
					_, err = f.WriteString("Wrong JSON")
					return err

				},
				wantErr: true,
				err:     "saving checksum to file: unmarshalling json: invalid character",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				ctx := context.Background()
				if c.prepareFunc != nil {
					err = c.prepareFunc()
					require.NoError(t, err)
				}
				err := DownloadAndUnpackImage(ctx, c.image, c.directory, filepath.Join(c.directory, "cache"), c.rc)
				// temporary quick fix for rate limiter issue
				if err != nil && strings.Contains(err.Error(), "You have reached your unauthenticated pull rate limit") {
					return
				}
				if !c.wantErr {
					require.NoError(t, err)
					require.DirExists(t, filepath.Join(c.directory, "cache"))
				} else {
					require.Error(t, err)
					require.Contains(t, err.Error(), c.err)
				}

			})
		}
	})
}

func TestRestoreImageFromTarGz(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	err = DownloadAndUnpackImage(context.Background(), "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e", testDir, filepath.Join(testDir, "cache"), RegistryConfig{scheme: "HTTPS", registry: "docker.io"})
	require.NoError(t, err)
	cachePath := filepath.Join(testDir, "sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e")
	require.FileExists(t, cachePath)

	t.Run("restoreImageFromTarGz tests", func(t *testing.T) {
		cases := []struct {
			title       string
			path        string
			prepareFunc func() error
			wantErr     bool
			err         string
		}{
			{
				title:   "Success",
				path:    cachePath,
				wantErr: false,
			},
			{
				title: "Unparsable tarball, failure",
				path:  cachePath,
				prepareFunc: func() error {
					_ = os.Remove(cachePath)
					_, err := os.Create(cachePath)
					return err
				},
				wantErr: true,
				err:     "parsing tarball",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				if c.prepareFunc != nil {
					err = c.prepareFunc()
					require.NoError(t, err)
				}
				_, err := restoreImageFromTarGz(c.path, nil)
				// temporary quick fix for rate limiter issue
				if err != nil && strings.Contains(err.Error(), "You have reached your unauthenticated pull rate limit") {
					return
				}
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

func TestPullImage(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "dhctltests")
	err := os.MkdirAll(testDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})

	t.Run("pullImage tests", func(t *testing.T) {
		cases := []struct {
			title       string
			imgRef      string
			rc          *RegistryConfig
			destDir     string
			prepareFunc func() error
			wantErr     bool
			err         string
		}{
			{
				title:   "Success",
				imgRef:  "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e",
				rc:      &RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				destDir: testDir,
				wantErr: false,
			},
			{
				title:   "Unaccessible image, failure",
				imgRef:  "docker.io/library/nginx:notatag100500",
				rc:      &RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				destDir: testDir,
				wantErr: true,
				err:     "pulling image",
			},
			{
				title:   "Wrong images_hash.json, failure",
				imgRef:  "docker.io/library/nginx@sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e",
				rc:      &RegistryConfig{scheme: "HTTPS", registry: "docker.io"},
				destDir: testDir,
				prepareFunc: func() error {
					if err = os.RemoveAll(filepath.Join(testDir, "sha256:5b4900b042ccfa8b0a73df622c3a60f2322faeb2be800cbee5aa7b44d241649e")); err != nil {
						return err
					}
					if err = os.RemoveAll(filepath.Join(testDir, "images_hashs.json")); err != nil {
						return err
					}
					f, err := os.Create(filepath.Join(testDir, "images_hashs.json"))
					if err != nil {
						return err
					}
					_, err = f.WriteString("Wrong JSON")
					return err

				},
				wantErr: true,
				err:     "saving checksum to file: unmarshalling json: invalid character",
			},
		}

		for _, c := range cases {
			t.Run(c.title, func(t *testing.T) {
				if c.prepareFunc != nil {
					err = c.prepareFunc()
					require.NoError(t, err)
				}
				ref, err := name.ParseReference(c.imgRef)
				require.NoError(t, err)
				opts, err := getOptsFromRegistryConfig(ref, c.rc)
				require.NoError(t, err)

				_, err = pullImage(context.Background(), ref, opts, ref.Identifier(), c.destDir, filepath.Join(c.destDir, "cache"))
				// temporary quick fix for rate limiter issue
				if err != nil && strings.Contains(err.Error(), "You have reached your unauthenticated pull rate limit") {
					return
				}
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
