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

package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func TestRenderAndSavePreflightReverseTunnelReachableScript(t *testing.T) {
	// RenderAndSaveTemplate reads the template directly from disk. In the install
	// container CandiDir resolves to options.DefaultCandiDir (/deckhouse/candi);
	// when running the unit test we point it at the candi dir of this repo.
	candiDir := options.DefaultCandiDir
	if _, err := os.Stat(candiDir); err != nil {
		candiDir, err = filepath.Abs(filepath.Join("..", "..", "..", "candi"))
		require.NoError(t, err)
	}

	path, err := RenderAndSavePreflightReverseTunnelReachableScript(
		t.Context(),
		"http://127.0.0.1:4282/healthz",
		&options.GlobalOptions{CandiDir: candiDir},
	)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	s := string(content)

	require.Contains(t, s, `target='http://127.0.0.1:4282/healthz'`)
	require.Contains(t, s, `target="${target#http://}"`)
	require.Contains(t, s, `minget "$target" --timeout 5 >/dev/null`)

	require.NotContains(t, s, "check_python")
	require.NotContains(t, s, "python_binary")
	require.NotContains(t, s, "urllib")
	require.NotContains(t, s, `minget "$target" --fail`)
}
