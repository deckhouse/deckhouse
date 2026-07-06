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

package machineclass

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoModulesDir is the real modules/ root relative to this package directory
// (8 levels up), so the resolver test exercises the actual provider layout
// modules/030-cloud-provider-<type>/cloud-instance-manager/machine-class.checksum.
const repoModulesDir = "../../../../../../../.."

func TestResolveChecksumTemplatePath_FindsRealMCMProvider(t *testing.T) {
	path := ResolveChecksumTemplatePath([]string{repoModulesDir}, FallbackTemplateBaseDir, "aws", MCMChecksumSubPath)

	_, err := os.Stat(path)
	require.NoError(t, err, "resolver must point at the real aws MCM template")
	assert.Equal(t, "machine-class.checksum", filepath.Base(path))
}

// The CAPI mode uses a different template (capi/instance-class.checksum) rendered
// by the same engine; the resolver must locate it for a real provider.
func TestResolveChecksumTemplatePath_FindsRealCAPIProvider(t *testing.T) {
	path := ResolveChecksumTemplatePath([]string{repoModulesDir}, FallbackTemplateBaseDir, "yandex", CAPIChecksumSubPath)

	_, err := os.Stat(path)
	require.NoError(t, err, "resolver must point at the real yandex CAPI template")
	assert.Equal(t, "instance-class.checksum", filepath.Base(path))
}

// End-to-end: resolve the real aws template by cloud type and render it, proving
// the resolver feeds RenderChecksum a valid template.
func TestReadChecksumTemplate_RendersResolvedProvider(t *testing.T) {
	content, err := ReadChecksumTemplate([]string{repoModulesDir}, FallbackTemplateBaseDir, "aws", MCMChecksumSubPath)
	require.NoError(t, err)

	got, err := RenderChecksum(content, map[string]interface{}{
		"instanceClass":   map[string]interface{}{"instanceType": "m5.large"},
		"manualRolloutID": "",
	})
	require.NoError(t, err)
	assert.Len(t, got, 64)
}

// An empty cloud type is the "not a cloud cluster" signal; reading must fail fast
// rather than resolve a bogus 030-cloud-provider- path.
func TestReadChecksumTemplate_EmptyCloudType(t *testing.T) {
	_, err := ReadChecksumTemplate(DefaultTemplateBaseDirs, FallbackTemplateBaseDir, "", MCMChecksumSubPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloud type not set")
}

// When no base dir contains the provider, the resolver returns the unchecked
// fallback path (mirroring the hook); the read then fails with a clear message.
func TestResolveChecksumTemplatePath_FallbackWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	path := ResolveChecksumTemplatePath([]string{tmp}, "/nonexistent/cloud-providers", "unknownprovider", MCMChecksumSubPath)
	assert.Equal(t, filepath.Join("/nonexistent/cloud-providers", "unknownprovider", "machine-class.checksum"), path)

	_, err := ReadChecksumTemplate([]string{tmp}, "/nonexistent/cloud-providers", "unknownprovider", MCMChecksumSubPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read checksum template")
}
