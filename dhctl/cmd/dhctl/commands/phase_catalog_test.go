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

package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// TestDefinePhaseCatalogCommand asserts the CLI command emits valid JSON
// matching the shared loader output — so the CLI and gRPC transports stay in
// sync with the single source of truth.
func TestDefinePhaseCatalogCommand(t *testing.T) {
	t.Parallel()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	app := kingpin.New("dhctl", "test")
	DefinePhaseCatalogCommand(app.Command("phase-catalog", "test"), options.New())

	_, err = app.Parse([]string{"phase-catalog"})
	os.Stdout = origStdout
	require.NoError(t, err)
	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)

	var got phases.TitlesCatalog
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(out), &got))

	titles, err := phases.LoadTitles()
	require.NoError(t, err)

	expected := titles.ToCatalog()
	assert.Equal(t, expected.Phases, got.Phases)
	assert.Equal(t, expected.SubPhases, got.SubPhases)
}
