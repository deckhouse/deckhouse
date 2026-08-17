/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package distribution

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

type countingRestarter struct {
	restarts int
	err      error
}

func (c *countingRestarter) Restart() error {
	c.restarts++
	return c.err
}

func newApplier(t *testing.T, restarter Restarter) *Applier {
	t.Helper()

	dir := t.TempDir()
	return &Applier{
		ConfigPath:     filepath.Join(dir, "config", "config.yaml"),
		UpstreamCAPath: filepath.Join(dir, "pki", "upstream-registry-ca.crt"),
		Restarter:      restarter,
		Options:        testOptions(),
	}
}

func upstreamSpec(host, ca string) *registryv1alpha1.RegistryStorageSpec {
	spec := &registryv1alpha1.RegistryStorageSpec{
		Upstream: &registryv1alpha1.Upstream{
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTPS,
				Host:   host,
				Path:   "/deckhouse/ee",
				Auth:   &registryv1alpha1.Auth{Username: "license-token", Password: "key"},
			},
		},
	}
	spec.Upstream.CA = ca
	return spec
}

// TestApplyConfiguresBothInstances covers the pair: the one the cluster pulls through, and the one a
// push arrives at. Both are rendered from the same spec, so they cannot drift on authentication or on
// where the blobs live — and exactly one of them proxies, which is what makes the other writable.
func TestApplyConfiguresBothInstances(t *testing.T) {
	applier := newApplier(t, &countingRestarter{})
	applier.WriteConfigPath = filepath.Join(t.TempDir(), "config-push", "config.yaml")

	_, err := applier.Apply(upstreamSpec("registry.deckhouse.io", ""))
	require.NoError(t, err)

	// Parsed rather than grepped: "proxy" also appears under `auth.token.proxy`, which is how the
	// registry reaches the token service beside it. A substring check passes on that and says nothing
	// about pull-through — it failed this test into a false negative on the first run.
	readConfig := func(path string) map[string]any {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, yaml.Unmarshal(raw, &parsed))
		return parsed
	}

	serving := readConfig(applier.ConfigPath)
	assert.Contains(t, serving, "proxy",
		"the serving instance caches misses from the upstream")

	writing := readConfig(applier.WriteConfigPath)
	writingProxy, _ := writing["proxy"].(map[string]any)
	assert.NotContains(t, writingProxy, "remoteurl",
		"a proxying registry refuses every write, which is why the push has an instance of its own")
	assert.Equal(t, true, writingProxy["skipmodecleanup"],
		"this is the instance that used to delete the store every time it started")

	// Both keep the same authentication and the same blob directory: they are one registry seen from
	// two sides, and a difference there would let a push land somewhere the cluster does not read.
	assert.Equal(t, serving["auth"], writing["auth"], "the two instances authenticate identically")
	assert.Equal(t, serving["storage"], writing["storage"], "the two instances share the blob directory")
}

// TestApplyWithoutASecondInstanceWritesNothingExtra keeps a syncer working beside an older pod
// template, where there is no second container to read that file.
func TestApplyWithoutASecondInstanceWritesNothingExtra(t *testing.T) {
	applier := newApplier(t, &countingRestarter{})
	require.Empty(t, applier.WriteConfigPath)

	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", ""))
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestApplyWritesAndRestarts(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)

	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "-----BEGIN CERTIFICATE-----upstream"))
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 1, restarter.restarts)

	config, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(config), "registry.deckhouse.io")

	// The configuration names the certificate authority file, so the two have to be
	// written together or the registry starts pointing at a path that is not there.
	certificate, err := os.ReadFile(applier.UpstreamCAPath)
	require.NoError(t, err)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----upstream", string(certificate))
}

// TestApplyIsIdempotent is the property that keeps the storage available: the
// reconciliation loop re-renders constantly, and a restart on every pass would
// leave the registry permanently down rather than merely reconfigured.
func TestApplyIsIdempotent(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)
	spec := upstreamSpec("registry.deckhouse.io", "ca")

	for range 5 {
		_, err := applier.Apply(spec)
		require.NoError(t, err)
	}

	assert.Equal(t, 1, restarter.restarts, "only the first pass changed anything")
}

// TestApplyRestartsOnACredentialChange is the mechanism behind the "expired
// upstream credentials while the operator is down" story.
func TestApplyRestartsOnACredentialChange(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)

	spec := upstreamSpec("registry.deckhouse.io", "ca")
	_, err := applier.Apply(spec)
	require.NoError(t, err)

	spec.Upstream.Auth.Password = "the-renewed-license-key"
	changed, err := applier.Apply(spec)
	require.NoError(t, err)

	assert.True(t, changed)
	assert.Equal(t, 2, restarter.restarts)

	config, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(config), "the-renewed-license-key")
}

func TestApplyRestartsOnACertificateAuthorityChange(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)

	_, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "old-ca"))
	require.NoError(t, err)

	// The configuration text does not change when only the certificate authority
	// does, since it merely names the file. Detecting this needs the file itself to
	// be compared.
	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "new-ca"))
	require.NoError(t, err)

	assert.True(t, changed)
	assert.Equal(t, 2, restarter.restarts)
}

// TestApplyRemovesAStaleCertificateAuthority matters because a leftover file would
// keep being trusted after the authority was removed from the configuration.
func TestApplyRemovesAStaleCertificateAuthority(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)

	_, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "ca"))
	require.NoError(t, err)
	require.FileExists(t, applier.UpstreamCAPath)

	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", ""))
	require.NoError(t, err)
	assert.True(t, changed)

	_, err = os.Stat(applier.UpstreamCAPath)
	assert.True(t, os.IsNotExist(err), "a removed certificate authority must not stay trusted")
}

// TestApplyGoingAirGap covers the transition through the file on disk: the pull-through
// section disappears, so the cache can no longer reach out.
//
// Asserted on the parsed document rather than on the text, because the configuration has
// two unrelated keys called `proxy` — the pull-through cache at the top level, and the
// token fetch under `auth.token`. That is docker distribution's own naming, and a
// substring check cannot tell the difference between the section that must go and the one
// that must stay.
func TestApplyGoingAirGap(t *testing.T) {
	restarter := &countingRestarter{}
	applier := newApplier(t, restarter)

	_, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "ca"))
	require.NoError(t, err)

	changed, err := applier.Apply(&registryv1alpha1.RegistryStorageSpec{})
	require.NoError(t, err)
	assert.True(t, changed)

	raw, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &config))

	// The section itself stays — it carries `skipmodecleanup`, without which the registry deletes the
	// whole store when it starts in a mode other than the one that last wrote it. What must be gone is
	// the address, which is what makes it a pull-through cache.
	proxy, _ := config["proxy"].(map[string]any)
	assert.NotContains(t, proxy, "remoteurl", "the cache can still reach out to an upstream")
	assert.NotContains(t, string(raw), "registry.deckhouse.io")

	// The token fetch stays, and has to: without it the token service would have to be
	// reachable by every client the registry is, and it listens on loopback.
	auth, _ := config["auth"].(map[string]any)
	token, _ := auth["token"].(map[string]any)
	assert.Contains(t, token, "proxy")
}

func TestApplyReportsAFailedRestart(t *testing.T) {
	restarter := &countingRestarter{err: errors.New("no registry process found")}
	applier := newApplier(t, restarter)

	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", "ca"))

	require.Error(t, err)
	// The change is reported as applied because it IS on disk: the next pass, or a
	// crash of the registry container, picks it up.
	assert.True(t, changed)

	config, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(config), "registry.deckhouse.io")
}

func TestApplyWithoutARestarter(t *testing.T) {
	applier := newApplier(t, nil)

	changed, err := applier.Apply(upstreamSpec("registry.deckhouse.io", ""))
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestWriteIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file")

	changed, err := writeIfChanged(path, []byte("first"), 0o600)
	require.NoError(t, err)
	assert.True(t, changed, "a file that did not exist counts as changed")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	changed, err = writeIfChanged(path, []byte("first"), 0o600)
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = writeIfChanged(path, []byte("second"), 0o600)
	require.NoError(t, err)
	assert.True(t, changed)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second", string(content))
}

// TestWriteIfChangedLeavesNoTemporaryFiles matters because the directory is read by
// the registry: a stray half-written file next to the configuration is at best
// confusing and at worst loaded.
func TestWriteIfChangedLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	for i := range 3 {
		_, err := writeIfChanged(path, []byte{byte(i)}, 0o600)
		require.NoError(t, err)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "config.yaml", entries[0].Name())
}

// TestApplyReadOnlyIsNotPartOfTheDesiredState is what keeps a crashed garbage collection from
// leaving a replica unable to accept writes forever: read-only is a condition a replica puts
// itself in, and the next ordinary Apply takes it back out.
func TestApplyReadOnlyIsNotPartOfTheDesiredState(t *testing.T) {
	applier := newApplier(t, &countingRestarter{})
	spec := upstreamSpec("registry.deckhouse.io", "")

	_, err := applier.Apply(spec)
	require.NoError(t, err)
	require.False(t, readOnly(t, applier), "writes were refused before anything asked for it")

	changed, err := applier.ApplyReadOnly(spec)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, readOnly(t, applier))

	// The same desired state as the first call, and it must undo the condition rather than
	// see no difference.
	changed, err = applier.Apply(spec)
	require.NoError(t, err)
	assert.True(t, changed, "the read-only condition was left in place")
	assert.False(t, readOnly(t, applier))
}

// TestApplyReadOnlyKeepsServingReads: the point of read-only over stopping the process is
// that a replica in this state still answers for every image it holds.
func TestApplyReadOnlyKeepsServingReads(t *testing.T) {
	applier := newApplier(t, &countingRestarter{})

	_, err := applier.ApplyReadOnly(upstreamSpec("registry.deckhouse.io", ""))
	require.NoError(t, err)

	raw, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &config))

	// Still listening, and still a pull-through cache: what changed is only that writes
	// are refused.
	assert.Contains(t, config, "http")
	assert.Contains(t, config, "proxy")
}

func readOnly(t *testing.T, applier *Applier) bool {
	t.Helper()

	raw, err := os.ReadFile(applier.ConfigPath)
	require.NoError(t, err)

	var config struct {
		Storage struct {
			Maintenance struct {
				ReadOnly struct {
					Enabled bool `json:"enabled"`
				} `json:"readonly"`
			} `json:"maintenance"`
		} `json:"storage"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &config))
	return config.Storage.Maintenance.ReadOnly.Enabled
}
