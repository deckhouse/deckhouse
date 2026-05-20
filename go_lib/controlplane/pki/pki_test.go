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

package pki

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allExpectedFiles is the complete set of files produced by CreatePKIBundle
// with the default cert tree scheme (local etcd).
var allExpectedFiles = []string{
	"ca.crt", "ca.key",
	"apiserver.crt", "apiserver.key",
	"apiserver-kubelet-client.crt", "apiserver-kubelet-client.key",
	"front-proxy-ca.crt", "front-proxy-ca.key",
	"front-proxy-client.crt", "front-proxy-client.key",
	"etcd/ca.crt", "etcd/ca.key",
	"etcd/server.crt", "etcd/server.key",
	"etcd/peer.crt", "etcd/peer.key",
	"etcd/healthcheck-client.crt", "etcd/healthcheck-client.key",
	"apiserver-etcd-client.crt", "apiserver-etcd-client.key",
	"sa.key", "sa.pub",
}

func TestCreatePKIBundle_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	rep, err := CreatePKIBundle(
		"test-node",
		"cluster.local",
		net.ParseIP("10.0.0.1"),
		"10.96.0.0/12",
		WithPKIDir(dir),
	)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 11)
	for _, e := range rep.Entries {
		assert.Equal(t, PKIActionWrittenCreated, e.Action, "artifact %q", e.Name)
	}

	for _, f := range allExpectedFiles {
		path := filepath.Join(dir, f)
		info, err := os.Stat(path)
		require.NoError(t, err, "expected file to exist: %s", f)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "file %s should have 0600 permissions", f)
	}
}

func TestCreatePKIBundle_Idempotent(t *testing.T) {
	dir := t.TempDir()
	opts := []configOption{WithPKIDir(dir)}
	args := []any{"test-node", "cluster.local", net.ParseIP("10.0.0.1"), "10.96.0.0/12"}

	rep1, err := CreatePKIBundle(args[0].(string), args[1].(string), args[2].(net.IP), args[3].(string), opts...)
	require.NoError(t, err)
	require.Len(t, rep1.Entries, 11)
	for _, e := range rep1.Entries {
		assert.Equal(t, PKIActionWrittenCreated, e.Action, "first run: %q", e.Name)
	}

	before := readAllFiles(t, dir, allExpectedFiles)

	rep2, err := CreatePKIBundle(args[0].(string), args[1].(string), args[2].(net.IP), args[3].(string), opts...)
	require.NoError(t, err)
	require.Len(t, rep2.Entries, 11)
	for _, e := range rep2.Entries {
		assert.Equal(t, PKIActionUnchanged, e.Action, "second run: %q", e.Name)
	}

	after := readAllFiles(t, dir, allExpectedFiles)

	for _, f := range allExpectedFiles {
		assert.Equal(t, before[f], after[f], "file %s should not change on second call", f)
	}
}

func TestCreatePKIBundle_CustomScheme_EtcdOnly(t *testing.T) {
	dir := t.TempDir()

	// Simulate etcd-arbiter mode: only etcd certificates are needed.
	etcdOnlyScheme := certTreeScheme{
		EtcdCACertName: {
			EtcdServerCertName,
			EtcdPeerCertName,
			EtcdHealthcheckClientCertName,
		},
	}

	rep, err := CreatePKIBundle(
		"test-node",
		"cluster.local",
		net.ParseIP("10.0.0.1"),
		"10.96.0.0/12",
		WithPKIDir(dir),
		WithCertTreeScheme(etcdOnlyScheme),
	)
	require.NoError(t, err)
	// One etcd CA, three leaf certs, SA key pair.
	require.Len(t, rep.Entries, 5)
	for _, e := range rep.Entries {
		assert.Equal(t, PKIActionWrittenCreated, e.Action, "artifact %q", e.Name)
	}

	// Etcd files must be present.
	for _, f := range []string{
		"etcd/ca.crt", "etcd/ca.key",
		"etcd/server.crt", "etcd/server.key",
		"etcd/peer.crt", "etcd/peer.key",
		"etcd/healthcheck-client.crt", "etcd/healthcheck-client.key",
	} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoError(t, err, "expected etcd file to exist: %s", f)
	}

	// SA key pair is always created regardless of the cert tree scheme.
	for _, f := range []string{"sa.key", "sa.pub"} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoError(t, err, "expected SA file to exist: %s", f)
	}

	// Kubernetes CA and apiserver files must NOT be created.
	for _, f := range []string{"ca.crt", "ca.key", "apiserver.crt", "front-proxy-ca.crt"} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.True(t, os.IsNotExist(err), "file %s should not exist in etcd-only scheme", f)
	}
}

func readAllFiles(t *testing.T, dir string, files []string) map[string][]byte {
	t.Helper()
	result := make(map[string][]byte, len(files))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		require.NoError(t, err, "failed to read %s", f)
		result[f] = data
	}
	return result
}
