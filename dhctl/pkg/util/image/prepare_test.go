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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The templates are patched while the image still sits in its staging directory, but the path
// written into them must be the one they are read from after the rename.
func TestPrepareFilesWritesTheFinalPath(t *testing.T) {
	staging := filepath.Join(t.TempDir(), "deckhouse")
	final := "/var/lib/dhctl/candi@sha256:x/deckhouse"

	tpl := filepath.Join(staging, "candi", "bashible", "step.sh.tpl")
	require.NoError(t, os.MkdirAll(filepath.Dir(tpl), 0o755))
	require.NoError(t, os.WriteFile(tpl, []byte("source deckhouse/candi/bashible/lib.sh\n"), 0o644))

	require.NoError(t, PrepareFiles(t.Context(), staging, final))

	content, err := os.ReadFile(tpl)
	require.NoError(t, err)
	require.Equal(t, "source "+final+"/candi/bashible/lib.sh\n", string(content))
}
