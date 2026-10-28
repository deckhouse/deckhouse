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
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

func phaseCatalogExec(t *testing.T) phases.TitlesCatalog {
	var out bytes.Buffer

	cmd := &cobra.Command{Use: "phase-catalog"}
	DefinePhaseCatalogCommand(cmd, options.New())
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())

	var ret phases.TitlesCatalog
	require.NoError(t, json.Unmarshal(out.Bytes(), &ret))

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
	assert.Equal(t, "Install Deckhouse controller", subTitle)

	phaseTitle = got.Phases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
	subTitle = got.SubPhases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
	assert.NotEmpty(t, phaseTitle)
	assert.NotEmpty(t, subTitle)
}

func TestDefinePhaseCatalogCommand(t *testing.T) {
	titles, err := phases.LoadTitles()
	require.NoError(t, err)

	got := phaseCatalogExec(t)

	expected := titles.ToCatalog()
	assert.EqualValues(t, expected, got)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// A catalog that could not be written is a failed run: a reader of a closed pipe gets nothing.
func TestPhaseCatalogWriteError(t *testing.T) {
	cmd := &cobra.Command{Use: "phase-catalog"}
	DefinePhaseCatalogCommand(cmd, options.New())
	cmd.SetOut(failingWriter{})
	cmd.SetArgs([]string{})
	require.ErrorContains(t, cmd.Execute(), "broken pipe")
}
