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

package bashible_tests

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

// renderPhase1Lib renders lib.sh the way the cloud-init script carries it: the bb-minget part,
// for a node that is not yet a Node.
func renderPhase1Lib(t *testing.T, values map[string]any) string {
	t.Helper()
	content, err := os.ReadFile(bashibleLib)
	require.NoError(t, err)
	content = append(content, []byte("\n{{ template \"bb-minget\" . }}\n")...)
	ctx := bashibleContext("ClusterBootstrap", map[string]any{"rppGet": "sha256:0"})
	for key, value := range values {
		ctx[key] = value
	}
	rendered, err := template.RenderTemplate(filepath.Base(bashibleLib), content, ctx)
	require.NoError(t, err)
	return rendered.Content.String()
}

// minget travels gzipped, because the script is the cloud-init user data and EC2 caps it.
func TestMingetInstallUnpacksTheGzippedBinary(t *testing.T) {
	chdirToRepositoryRoot(t)
	binary := "#!/bin/sh\necho minget runs\n"
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, err := zw.Write([]byte(binary))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "minget")
	lib := renderPhase1Lib(t, map[string]any{"mingetB64": base64.StdEncoding.EncodeToString(gz.Bytes())})
	lib = strings.ReplaceAll(lib, "/opt/deckhouse/bin/minget", path)

	out, err := exec.Command("bash", "-c", "set -Eeo pipefail\n"+lib+"\nbb-minget-install\n"+path).CombinedOutput()
	require.NoError(t, err, string(out))
	require.Equal(t, "minget runs\n", string(out))
	written, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, binary, string(written))
}
