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

package checks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// candiOptionsFor gives a check that renders a script somewhere to render it
// from. Without options the render dereferences a nil pointer, and a panic in
// one test takes the whole package's binary down with it.
func candiOptionsFor(t *testing.T, scripts ...string) *options.GlobalOptions {
	t.Helper()

	candiDir := t.TempDir()
	dir := filepath.Join(candiDir, "bashible", "preflight")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, script := range scripts {
		require.NoError(t, os.WriteFile(filepath.Join(dir, script), []byte("#!/bin/bash\n"), 0o644))
	}

	return &options.GlobalOptions{CandiDir: candiDir}
}
