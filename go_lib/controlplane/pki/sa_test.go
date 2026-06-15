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
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSAKeysIfNotExists_CreatesNewPair(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	var rep PKIApplyReport
	err := createSAKeysIfNotExists(cfg, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, "sa", rep.Entries[0].Name)
	assert.Equal(t, PKIEntryKindServiceAccountKeys, rep.Entries[0].Kind)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	for _, f := range []string{"sa.key", "sa.pub"} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoError(t, err, "expected %s to exist", f)
	}
}

func TestCreateSAKeysIfNotExists_SkipsWhenBothExist(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	var rep PKIApplyReport
	err := createSAKeysIfNotExists(cfg, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	keyBefore, err := os.ReadFile(filepath.Join(dir, "sa.key"))
	require.NoError(t, err)
	pubBefore, err := os.ReadFile(filepath.Join(dir, "sa.pub"))
	require.NoError(t, err)

	// Second call must leave both files untouched.
	rep = PKIApplyReport{}
	err = createSAKeysIfNotExists(cfg, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionUnchanged, rep.Entries[0].Action)

	keyAfter, err := os.ReadFile(filepath.Join(dir, "sa.key"))
	require.NoError(t, err)
	pubAfter, err := os.ReadFile(filepath.Join(dir, "sa.pub"))
	require.NoError(t, err)

	assert.Equal(t, keyBefore, keyAfter, "sa.key should not change on second call")
	assert.Equal(t, pubBefore, pubAfter, "sa.pub should not change on second call")
}

func TestCreateSAKeysIfNotExists_RestoresMissingPub(t *testing.T) {
	dir := t.TempDir()
	cfg := makeTestConfig(t, dir)

	// Create the full pair first.
	var rep PKIApplyReport
	err := createSAKeysIfNotExists(cfg, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionWrittenCreated, rep.Entries[0].Action)

	// Simulate sa.pub going missing (e.g. accidental deletion).
	err = os.Remove(filepath.Join(dir, "sa.pub"))
	require.NoError(t, err)

	// Must restore sa.pub from the existing sa.key.
	rep = PKIApplyReport{}
	err = createSAKeysIfNotExists(cfg, &rep)
	require.NoError(t, err)
	require.Len(t, rep.Entries, 1)
	assert.Equal(t, PKIActionSAPublicKeyRestored, rep.Entries[0].Action)

	pubData, err := os.ReadFile(filepath.Join(dir, "sa.pub"))
	require.NoError(t, err, "sa.pub should have been restored")

	// Verify the restored file contains a parseable PKIX public key.
	block, _ := pem.Decode(pubData)
	require.NotNil(t, block, "sa.pub should be a valid PEM file")
	_, err = x509.ParsePKIXPublicKey(block.Bytes)
	assert.NoError(t, err, "sa.pub should contain a valid PKIX public key")
}
