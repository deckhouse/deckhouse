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

package phases

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allPhases() map[string]struct{} {
	ret := make(map[string]struct{})

	for _, operationPhases := range allOperationPhases() {
		for _, ph := range operationPhases {
			ret[string(ph.Phase)] = struct{}{}
		}
	}

	return ret
}

func allSubPhases() map[string]struct{} {
	ret := make(map[string]struct{})

	for _, phaseList := range allOperationPhases() {
		for _, ph := range phaseList {
			for _, sub := range ph.SubPhases {
				ret[string(sub)] = struct{}{}
			}
		}
	}

	return ret
}

func TestTitles_Load(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)
	require.NotNil(t, titles)

	for _, lang := range Languages {
		require.Contains(t, titles.phase, lang, "phase namespace missing %q language", lang)
		require.NotEmpty(t, titles.phase[lang], "phase namespace empty for %q language", lang)

		require.Contains(t, titles.subPhase, lang, "subphase namespace missing %q language", lang)
		require.NotEmpty(t, titles.subPhase[lang], "subphase namespace empty for %q language", lang)
	}
}

func TestTitles_Load_Coverage(t *testing.T) {
	t.Parallel()

	phases := allPhases()
	subPhases := allSubPhases()
	titles, err := LoadTitles()
	require.NoError(t, err)

	// Every emitted phase code must resolve to a non-empty title in every language.
	for phase := range phases {
		for _, lang := range Languages {
			title, ok := titles.phase[lang][OperationPhase(phase)]
			require.True(t, ok, "phase %q has no %q translation", phase, lang)
			require.NotEmpty(t, title, "phase %q has empty %q title", phase, lang)
		}
	}

	// Every emitted subphase code must resolve to a non-empty title in every language.
	for subPhase := range subPhases {
		for _, lang := range Languages {
			title, ok := titles.subPhase[lang][OperationSubPhase(subPhase)]
			require.True(t, ok, "subphase %q has no %q translation", subPhase, lang)
			require.NotEmpty(t, title, "subphase %q has empty %q title", subPhase, lang)
		}
	}
}

func TestTitles_ToCatalog(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)

	expected := titles.ToCatalog()
	require.NotEmpty(t, expected.Phases)
	require.NotEmpty(t, expected.SubPhases)

	assert.Equal(t, "Install Deckhouse", expected.Phases[string(InstallDeckhousePhase)][string(ENLanguage)])
	assert.Equal(t, "Install...", expected.SubPhases[string(InstallDeckhouseSubPhaseInstall)][string(ENLanguage)])

	// ToCatalog returns defensive copies: mutating the result does not affect the source.
	original := expected.Phases[string(BaseInfraPhase)][string(ENLanguage)]
	expected.Phases[string(BaseInfraPhase)][string(ENLanguage)] = "tampered"
	assert.Equal(t, original, titles.Phase(BaseInfraPhase))
}

func TestTitles_ToCatalog_Coverage(t *testing.T) {
	t.Parallel()

	phases := allPhases()
	subPhases := allSubPhases()
	titles, err := LoadTitles()
	require.NoError(t, err)

	catalog := titles.ToCatalog()
	require.NotEmpty(t, catalog.Phases)
	require.NotEmpty(t, catalog.SubPhases)

	// Every emitted phase code must resolve to a non-empty title in every language.
	for phase := range phases {
		for _, lang := range Languages {
			title, ok := catalog.Phases[phase][string(lang)]
			require.True(t, ok, "phase %q has no %q translation", phase, lang)
			require.NotEmpty(t, title, "phase %q has empty %q title", phase, lang)
		}
	}

	// Every emitted subphase code must resolve to a non-empty title in every language.
	for subPhase := range subPhases {
		for _, lang := range Languages {
			title, ok := catalog.SubPhases[subPhase][string(lang)]
			require.True(t, ok, "subphase %q has no %q translation", subPhase, lang)
			require.NotEmpty(t, title, "subphase %q has empty %q title", subPhase, lang)
		}
	}
}

func TestTitles_Phase(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)

	// Known code returns the English title.
	assert.Equal(t, "Base Infrastructure", titles.Phase(BaseInfraPhase))
	assert.Equal(t, "Install Deckhouse", titles.Phase(InstallDeckhousePhase))

	// Unknown code falls back to the raw code.
	assert.Equal(t, "", titles.Phase(OperationPhase("NoSuchPhase")))
	assert.Equal(t, "", titles.Phase(OperationPhase("")))
}

func TestTitles_SubPhase(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)

	// Known code returns the English title.
	assert.Equal(t, "Install...", titles.SubPhase(InstallDeckhouseSubPhaseInstall))
	assert.Equal(t, "Connect to master host", titles.SubPhase(InstallDeckhouseSubPhaseConnect))

	// Unknown code falls back to the raw code.
	assert.Equal(t, "", titles.SubPhase(OperationSubPhase("NoSuchSubPhase")))
	assert.Equal(t, "", titles.SubPhase(OperationSubPhase("")))
}

func TestReadYamlFile(t *testing.T) {
	t.Parallel()

	t.Run("valid file", func(t *testing.T) {
		t.Parallel()

		target := make(map[OperationPhase]string)
		require.NoError(t, readYamlFile("i18n/phases.en.yaml", &target))
		require.NotEmpty(t, target)
		assert.Equal(t, "Base Infrastructure", target[BaseInfraPhase])
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		target := make(map[OperationPhase]string)
		err := readYamlFile("i18n/nonexistent.yaml", &target)
		require.Error(t, err)
		assert.Empty(t, target)
	})
}
