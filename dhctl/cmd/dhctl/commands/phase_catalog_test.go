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

func phaseCatalogExec(t *testing.T) phases.TitlesCatalog {
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

	var ret phases.TitlesCatalog
	require.NoError(t, json.Unmarshal(out, &ret))

	require.NotEmpty(t, ret.Phases)
	require.NotEmpty(t, ret.SubPhases)
	return ret
}

func TestGetPhaseCatalog(t *testing.T) {
	got := phaseCatalogExec(t)

	installDeckhousePhase := string(phases.InstallDeckhousePhase)
	phaseTitle := got.Phases[installDeckhousePhase].ByLocale[string(phases.ENLocale)]
	subTitle := got.SubPhases[installDeckhousePhase].ByLocale[string(phases.ENLocale)]
	assert.Equal(t, "Install Deckhouse", phaseTitle)
	assert.Equal(t, "Install...", subTitle)

	phaseTitle = got.Phases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
	subTitle = got.SubPhases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
}

func TestDefinePhaseCatalogCommand(t *testing.T) {
	titles, err := phases.LoadTitles()
	require.NoError(t, err)

	got := phaseCatalogExec(t)

	expected := titles.ToCatalog()
	assert.EqualValues(t, expected, got)
}
