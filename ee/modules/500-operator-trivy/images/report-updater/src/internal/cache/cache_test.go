/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const bduImage = "harbor.example.com/deckhouse/cse/security/trivy-bdu:1"

func encodeAuth(user, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
}

// encodeMalformedAuth returns an auth field that does not decode into a
// `user:password` pair. It is assembled at runtime rather than written as a
// base64 literal, which secret scanners report as a generic API key.
func encodeMalformedAuth() string {
	return base64.StdEncoding.EncodeToString([]byte("not-a-pair"))
}

// newTestCache writes the given docker config into a temporary directory and
// returns a cache pointed at it, together with the buffer the logger writes to.
func newTestCache(t *testing.T, image, dockerConfig string) (*VulnerabilityCache, *bytes.Buffer) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, dockerConfigFilename), []byte(dockerConfig), 0o600); err != nil {
		t.Fatalf("write docker config: %v", err)
	}

	// The keychain reads $HOME/.docker/config.json before $DOCKER_CONFIG, so
	// point $HOME at an empty directory to keep the developer's own docker
	// config out of the test. t.Setenv restores both variables afterwards.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOCKER_CONFIG", "")
	if err := useDockerConfig(dir); err != nil {
		t.Fatalf("use docker config: %v", err)
	}

	ref, err := name.ParseReference(image, name.StrictValidation)
	if err != nil {
		t.Fatalf("parse reference: %v", err)
	}

	logs := &bytes.Buffer{}
	return &VulnerabilityCache{
		logger: log.New(logs, "", 0),
		dict:   Dictionary{Data: make(map[string][]string)},
		config: RegistryConfig{
			repository: ref.Context(),
			tag:        ref.Identifier(),
			configDir:  dir,
		},
	}, logs
}

func TestCredential(t *testing.T) {
	tests := []struct {
		name         string
		dockerConfig string
		image        string
		want         auth.Credential
	}{
		{
			name:         "auth field",
			dockerConfig: fmt.Sprintf(`{"auths":{"harbor.example.com":{"auth":%q}}}`, encodeAuth("deckhouse", "secret")),
			want:         auth.Credential{Username: "deckhouse", Password: "secret"},
		},
		{
			// The shape that broke report-updater on CSE 1.73.4: kubelet pulls
			// images with it, the hand-written parser saw no credentials at all.
			name:         "username and password without auth",
			dockerConfig: `{"auths":{"harbor.example.com":{"username":"deckhouse","password":"secret","email":""}}}`,
			want:         auth.Credential{Username: "deckhouse", Password: "secret"},
		},
		{
			// The platform registry of a cluster with the in-cluster registry
			// enabled is registry.d8-system.svc:5001, so the port is not exotic.
			name:         "key carries a port",
			dockerConfig: fmt.Sprintf(`{"auths":{"https://harbor.example.com:5001":{"auth":%q}}}`, encodeAuth("deckhouse", "secret")),
			image:        "harbor.example.com:5001/deckhouse/cse/security/trivy-bdu:1",
			want:         auth.Credential{Username: "deckhouse", Password: "secret"},
		},
		{
			name:         "key carries a scheme",
			dockerConfig: fmt.Sprintf(`{"auths":{"https://harbor.example.com":{"auth":%q}}}`, encodeAuth("deckhouse", "secret")),
			want:         auth.Credential{Username: "deckhouse", Password: "secret"},
		},
		{
			name:         "key carries a repository path",
			dockerConfig: fmt.Sprintf(`{"auths":{"harbor.example.com/deckhouse/cse":{"auth":%q}}}`, encodeAuth("deckhouse", "secret")),
			want:         auth.Credential{Username: "deckhouse", Password: "secret"},
		},
		{
			name:         "password contains a colon",
			dockerConfig: fmt.Sprintf(`{"auths":{"harbor.example.com":{"auth":%q}}}`, encodeAuth("deckhouse", "se:cr:et")),
			want:         auth.Credential{Username: "deckhouse", Password: "se:cr:et"},
		},
		{
			name:         "identity token",
			dockerConfig: `{"auths":{"harbor.example.com":{"identitytoken":"token"}}}`,
			want:         auth.Credential{RefreshToken: "token"},
		},
		{
			name:         "registry token",
			dockerConfig: `{"auths":{"harbor.example.com":{"registrytoken":"token"}}}`,
			want:         auth.Credential{AccessToken: "token"},
		},
		{
			name:         "entry for another registry",
			dockerConfig: fmt.Sprintf(`{"auths":{"registry.deckhouse.io":{"auth":%q}}}`, encodeAuth("deckhouse", "secret")),
			want:         auth.EmptyCredential,
		},
		{
			// The documented shape of an anonymous registry: the entry exists
			// and holds nothing.
			name:         "entry without credentials",
			dockerConfig: `{"auths":{"harbor.example.com":{}}}`,
			want:         auth.EmptyCredential,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := tt.image
			if image == "" {
				image = bduImage
			}
			cache, _ := newTestCache(t, image, tt.dockerConfig)

			got, err := cache.credential()
			if err != nil {
				t.Fatalf("credential(): %v", err)
			}
			if got != tt.want {
				t.Errorf("credential() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCredentialLogsAnonymousFallback(t *testing.T) {
	dockerConfig := fmt.Sprintf(
		`{"auths":{"registry.deckhouse.io":{"auth":%q},"other.example.com":{}}}`,
		encodeAuth("deckhouse", "secret"),
	)
	cache, logs := newTestCache(t, bduImage, dockerConfig)

	if _, err := cache.credential(); err != nil {
		t.Fatalf("credential(): %v", err)
	}

	logged := logs.String()
	for _, want := range []string{"anonymous access", "harbor.example.com", "registry.deckhouse.io", "other.example.com"} {
		if !strings.Contains(logged, want) {
			t.Errorf("log %q does not mention %q", logged, want)
		}
	}
}

func TestCredentialMalformedAuth(t *testing.T) {
	cache, _ := newTestCache(t, bduImage, fmt.Sprintf(`{"auths":{"harbor.example.com":{"auth":%q}}}`, encodeMalformedAuth()))

	if _, err := cache.credential(); err == nil {
		t.Fatal("credential() accepted an auth field that is not base64(user:password)")
	}
}

func TestUseDockerConfigMissingFile(t *testing.T) {
	t.Setenv("DOCKER_CONFIG", "")

	if err := useDockerConfig(t.TempDir()); err == nil {
		t.Fatal("useDockerConfig() accepted a directory without config.json")
	}
}

func TestDockerConfigHosts(t *testing.T) {
	cache, _ := newTestCache(t, bduImage, `{"auths":{"registry.deckhouse.io":{},"harbor.example.com":{}}}`)

	got := cache.dockerConfigHosts()
	want := []string{"harbor.example.com", "registry.deckhouse.io"}
	if len(got) != len(want) {
		t.Fatalf("dockerConfigHosts() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dockerConfigHosts() = %v, want %v", got, want)
		}
	}
}

func TestCredentialWarnsOnceAboutAnonymousAccess(t *testing.T) {
	cache, logs := newTestCache(t, bduImage, `{"auths":{"registry.deckhouse.io":{}}}`)

	for range 3 {
		if _, err := cache.credential(); err != nil {
			t.Fatalf("credential(): %v", err)
		}
	}

	if got := strings.Count(logs.String(), "using anonymous access"); got != 1 {
		t.Errorf("anonymous access reported %d times, want once:\n%s", got, logs.String())
	}
}

// TestCredentialRereadsRotatedConfig guards the promise that rotated
// credentials are picked up on the next renewal. The whole guarantee rests on
// the keychain reading the file on every Resolve, so a dependency that starts
// memoizing the config would otherwise break documented behaviour silently.
func TestCredentialRereadsRotatedConfig(t *testing.T) {
	before := fmt.Sprintf(`{"auths":{"harbor.example.com":{"auth":%q}}}`, encodeAuth("deckhouse", "old"))
	after := fmt.Sprintf(`{"auths":{"harbor.example.com":{"auth":%q}}}`, encodeAuth("deckhouse", "new"))

	cache, _ := newTestCache(t, bduImage, before)

	got, err := cache.credential()
	if err != nil {
		t.Fatalf("credential(): %v", err)
	}
	if want := (auth.Credential{Username: "deckhouse", Password: "old"}); got != want {
		t.Fatalf("credential() = %+v, want %+v", got, want)
	}

	path := filepath.Join(cache.config.configDir, dockerConfigFilename)
	if err = os.WriteFile(path, []byte(after), 0o600); err != nil {
		t.Fatalf("rotate docker config: %v", err)
	}

	got, err = cache.credential()
	if err != nil {
		t.Fatalf("credential() after rotation: %v", err)
	}
	if want := (auth.Credential{Username: "deckhouse", Password: "new"}); got != want {
		t.Errorf("credential() after rotation = %+v, want %+v", got, want)
	}
}

func TestCredentialNamesBrokenEntry(t *testing.T) {
	dockerConfig := fmt.Sprintf(
		`{"auths":{"harbor.example.com":{"auth":%q},"broken.example.com":{"auth":%q}}}`,
		encodeAuth("deckhouse", "secret"), encodeMalformedAuth(),
	)
	cache, _ := newTestCache(t, bduImage, dockerConfig)

	_, err := cache.credential()
	if err == nil {
		t.Fatal("credential() accepted a config with an undecodable auth field")
	}
	if !strings.Contains(err.Error(), "broken.example.com") {
		t.Errorf("error %q does not name the broken entry", err)
	}
}
