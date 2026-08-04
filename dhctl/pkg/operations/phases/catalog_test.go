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

func emittedPhases(t *testing.T) map[OperationPhase]struct{} {
	t.Helper()

	ret := make(map[OperationPhase]struct{})
	for _, ph := range allPhases() {
		ret[ph] = struct{}{}
	}
	for _, operationPhases := range allOperationPhases() {
		for _, ph := range operationPhases {
			ret[ph.Phase] = struct{}{}
		}
	}
	return ret
}

func emittedSubPhases(t *testing.T) map[OperationSubPhase]struct{} {
	t.Helper()

	ret := make(map[OperationSubPhase]struct{})
	for _, sp := range allSubPhases() {
		ret[sp] = struct{}{}
	}
	for _, operationPhases := range allOperationPhases() {
		for _, ph := range operationPhases {
			for _, sp := range ph.SubPhases {
				ret[sp] = struct{}{}
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

	for _, locale := range AllLocales() {
		assert.Contains(t, titles.phase, locale, "phase namespace missing %q locale", locale)
		assert.NotEmpty(t, titles.phase[locale], "phase namespace empty for %q locale", locale)

		assert.Contains(t, titles.subPhase, locale, "subphase namespace missing %q locale", locale)
		assert.NotEmpty(t, titles.subPhase[locale], "subphase namespace empty for %q locale", locale)
	}
}

// TestTitles_Load_Coverage asserts the two invariants that keep phase reporting and
// translations in sync.
func TestTitles_Load_Coverage(t *testing.T) {
	t.Parallel()

	phases := emittedPhases(t)
	subPhases := emittedSubPhases(t)
	titles, err := LoadTitles()
	require.NoError(t, err)

	// Forward: every emittable phase resolves to a non-empty title per locale.
	for phase := range phases {
		for _, locale := range AllLocales() {
			title, ok := titles.phase[locale][phase]
			assert.True(t, ok, "phase %q has no %q translation", phase, locale)
			assert.NotEmpty(t, title, "phase %q has empty %q title", phase, locale)
		}
	}

	// Forward: every emittable subphase resolves to a non-empty title per locale.
	for subPhase := range subPhases {
		for _, locale := range AllLocales() {
			title, ok := titles.subPhase[locale][subPhase]
			assert.True(t, ok, "subphase %q has no %q translation", subPhase, locale)
			assert.NotEmpty(t, title, "subphase %q has empty %q title", subPhase, locale)
		}
	}

	// Reverse: every YAML subphase key is a registered subphase.
	knownPhases := make(map[string]struct{}, len(phases))
	for ph := range phases {
		knownPhases[string(ph)] = struct{}{}
	}
	for _, locale := range AllLocales() {
		for phase := range titles.phase[locale] {
			_, ok := knownPhases[string(phase)]
			assert.True(t, ok, "phases.%s.yaml has stale key %q with no matching phase code", locale, phase)
		}
	}

	// Reverse: every YAML subphase key is a registered subphase.
	knownSubPhases := make(map[string]struct{}, len(subPhases))
	for sp := range subPhases {
		knownSubPhases[string(sp)] = struct{}{}
	}
	for _, locale := range AllLocales() {
		for sp := range titles.subPhase[locale] {
			_, ok := knownSubPhases[string(sp)]
			assert.True(t, ok, "subphases.%s.yaml has stale key %q with no matching subphase code", locale, sp)
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

	assert.Equal(t, "Install Deckhouse", expected.Phases[string(InstallDeckhousePhase)].ByLocale[string(ENLocale)])
	assert.Equal(t, "Install...", expected.SubPhases[string(InstallDeckhouseSubPhaseInstall)].ByLocale[string(ENLocale)])

	// ToCatalog returns defensive copies: mutating the result does not affect the source.
	original := expected.Phases[string(BaseInfraPhase)].ByLocale[string(ENLocale)]
	expected.Phases[string(BaseInfraPhase)].ByLocale[string(ENLocale)] = "tampered"
	assert.Equal(t, original, titles.Phase(BaseInfraPhase))
}

// TestTitles_ToCatalog_Coverage asserts the two invariants that keep phase reporting and
// translations in sync.
func TestTitles_ToCatalog_Coverage(t *testing.T) {
	t.Parallel()

	phases := emittedPhases(t)
	subPhases := emittedSubPhases(t)
	titles, err := LoadTitles()
	require.NoError(t, err)

	catalog := titles.ToCatalog()
	require.NotEmpty(t, catalog.Phases)
	require.NotEmpty(t, catalog.SubPhases)

	// Forward: every emittable phase resolves to a non-empty title per locale.
	for phase := range phases {
		for _, locale := range AllLocales() {
			title, ok := catalog.Phases[string(phase)].ByLocale[string(locale)]
			assert.True(t, ok, "catalog phase %q has no %q translation", phase, locale)
			assert.NotEmpty(t, title, "catalog phase %q has empty %q title", phase, locale)
		}
	}

	// Forward: every emittable subphase resolves to a non-empty title per locale.
	for subPhase := range subPhases {
		for _, locale := range AllLocales() {
			title, ok := catalog.SubPhases[string(subPhase)].ByLocale[string(locale)]
			assert.True(t, ok, "catalog subphase %q has no %q translation", subPhase, locale)
			assert.NotEmpty(t, title, "catalog subphase %q has empty %q title", subPhase, locale)
		}
	}

	// Reverse: every subphase key is a registered subphase.
	knownPhases := make(map[string]struct{}, len(phases))
	for ph := range phases {
		knownPhases[string(ph)] = struct{}{}
	}
	for phase := range catalog.Phases {
		_, ok := knownPhases[phase]
		assert.True(t, ok, "catalog phase key %q has no matching phase code", phase)
	}

	// Reverse: every subphase key is a registered subphase.
	knownSubPhases := make(map[string]struct{}, len(subPhases))
	for sp := range subPhases {
		knownSubPhases[string(sp)] = struct{}{}
	}
	for sp := range catalog.SubPhases {
		_, ok := knownSubPhases[sp]
		assert.True(t, ok, "catalog subphase key %q has no matching subphase code", sp)
	}
}

func TestTitles_Phase(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)

	assert.Equal(t, "Base Infrastructure", titles.Phase(BaseInfraPhase))
	assert.Equal(t, "Install Deckhouse", titles.Phase(InstallDeckhousePhase))

	assert.Equal(t, "", titles.Phase(OperationPhase("NoSuchPhase")))
	assert.Equal(t, "", titles.Phase(OperationPhase("")))
}

func TestTitles_SubPhase(t *testing.T) {
	t.Parallel()

	titles, err := LoadTitles()
	require.NoError(t, err)

	assert.Equal(t, "Install...", titles.SubPhase(InstallDeckhouseSubPhaseInstall))
	assert.Equal(t, "Connect to master host", titles.SubPhase(InstallDeckhouseSubPhaseConnect))

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
