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

const repoModulesDir = "../../../../../../../.."

func TestResolveChecksumTemplatePath_FindsRealMCMProvider(t *testing.T) {
	path := ResolveChecksumTemplatePath([]string{repoModulesDir}, FallbackTemplateBaseDir, "aws", MCMChecksumSubPath)

	_, err := os.Stat(path)
	require.NoError(t, err, "resolver must point at the real aws MCM template")
	assert.Equal(t, "machine-class.checksum", filepath.Base(path))
}

func TestResolveChecksumTemplatePath_FindsRealCAPIProvider(t *testing.T) {
	path := ResolveChecksumTemplatePath([]string{repoModulesDir}, FallbackTemplateBaseDir, "yandex", CAPIChecksumSubPath)

	_, err := os.Stat(path)
	require.NoError(t, err, "resolver must point at the real yandex CAPI template")
	assert.Equal(t, "instance-class.checksum", filepath.Base(path))
}

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

func TestReadChecksumTemplate_EmptyCloudType(t *testing.T) {
	_, err := ReadChecksumTemplate(DefaultTemplateBaseDirs, FallbackTemplateBaseDir, "", MCMChecksumSubPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloud type not set")
}

func TestResolveChecksumTemplatePath_FallbackWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	path := ResolveChecksumTemplatePath([]string{tmp}, "/nonexistent/cloud-providers", "unknownprovider", MCMChecksumSubPath)
	assert.Equal(t, filepath.Join("/nonexistent/cloud-providers", "unknownprovider", "machine-class.checksum"), path)

	_, err := ReadChecksumTemplate([]string{tmp}, "/nonexistent/cloud-providers", "unknownprovider", MCMChecksumSubPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read checksum template")
}
