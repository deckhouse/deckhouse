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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeNodeUsers(t *testing.T, provider, body string) string {
	t.Helper()

	dir := t.TempDir()
	providerDir := filepath.Join(dir, "cloud-providers", provider)
	require.NoError(t, os.MkdirAll(providerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(providerDir, "node-users.yml"), []byte(body), 0o644))

	return dir
}

// The file is the contract, so every cloud provider ships one. A provider without it says
// nothing about the users list dhctl assembles, and silence is what this whole file exists
// to prevent.
func TestLoadProviderDefaultUserRequiresTheFile(t *testing.T) {
	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	_, err := LoadProviderDefaultUser(m, candiDirOptions(t.TempDir()))

	require.ErrorContains(t, err, "read node users file")
}

// The marker is what ten providers out of eleven write: their images carry a distro user.
func TestLoadProviderDefaultUserWithDistroMarker(t *testing.T) {
	dir := writeNodeUsers(t, "demo", "schemaVersion: 1\ndefaultUser: default\n")

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	user, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.NoError(t, err)
	require.Nil(t, user, "the marker means cloud-init's own default user")
}

func TestLoadProviderDefaultUserRejectsAnUnknownMarker(t *testing.T) {
	dir := writeNodeUsers(t, "demo", "schemaVersion: 1\ndefaultUser: whoever\n")

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	_, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.ErrorContains(t, err, `defaultUser is "whoever"`)
}

func TestLoadProviderDefaultUserFromFile(t *testing.T) {
	dir := writeNodeUsers(t, "demo", `schemaVersion: 1
defaultUser:
  name: user
  groups: [users, wheel]
`)

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	user, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, "user", user.Name)
	require.Equal(t, []string{"users", "wheel"}, user.Groups)
}

// A file of a schema this dhctl does not know describes an account it cannot render, and
// rendering the wrong one leaves the node reachable by nobody.
func TestLoadProviderDefaultUserRejectsUnknownSchema(t *testing.T) {
	dir := writeNodeUsers(t, "demo", `schemaVersion: 2
defaultUser:
  name: user
`)

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	_, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.ErrorContains(t, err, "unsupported schemaVersion 2")
}

func TestLoadProviderDefaultUserRejectsNamelessAccount(t *testing.T) {
	dir := writeNodeUsers(t, "demo", `schemaVersion: 1
defaultUser:
  groups: [wheel]
`)

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	_, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.ErrorContains(t, err, "defaultUser has no name")
}

// A file that declares nothing leaves the reader guessing, which is the state this file
// replaces. The marker has to be written out.
func TestLoadProviderDefaultUserRequiresADeclaration(t *testing.T) {
	dir := writeNodeUsers(t, "demo", "schemaVersion: 1\n")

	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "demo"}

	_, err := LoadProviderDefaultUser(m, candiDirOptions(dir))

	require.ErrorContains(t, err, "defaultUser is required")
}

// A static cluster has no provider tree to read the file from.
func TestLoadProviderDefaultUserSkipsStaticCluster(t *testing.T) {
	user, err := LoadProviderDefaultUser(&MetaConfig{ClusterType: "Static"}, candiDirOptions(t.TempDir()))

	require.NoError(t, err)
	require.Nil(t, user)
}
