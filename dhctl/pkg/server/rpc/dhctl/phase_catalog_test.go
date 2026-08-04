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

package dhctl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

func phaseCatalogExec(t *testing.T) *pb.PhaseCatalog {
	s := New(ServiceParams{})
	ret, err := s.GetPhaseCatalog(context.Background(), &pb.PhaseCatalogRequest{})
	require.NoError(t, err)
	require.NotNil(t, ret)

	require.NotEmpty(t, ret.Phases)
	require.NotEmpty(t, ret.SubPhases)
	return ret
}

func TestGetPhaseCatalog(t *testing.T) {
	t.Parallel()

	got := phaseCatalogExec(t)

	installDeckhousePhase := string(phases.InstallDeckhousePhase)
	phaseTitle := got.Phases[installDeckhousePhase].ByLocale[string(phases.ENLocale)]
	subTitle := got.SubPhases[installDeckhousePhase].ByLocale[string(phases.ENLocale)]
	assert.Equal(t, "Install Deckhouse", phaseTitle)
	assert.Equal(t, "Install...", subTitle)

	phaseTitle = got.Phases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
	subTitle = got.SubPhases[installDeckhousePhase].ByLocale[string(phases.RULocale)]
}

func TestGetPhaseCatalog_MatchesLoader(t *testing.T) {
	t.Parallel()

	titles, err := phases.LoadTitles()
	require.NoError(t, err)
	catalog := titles.ToCatalog()

	got := phaseCatalogExec(t)

	for code, lt := range catalog.Phases {
		pbTitles, ok := got.Phases[code]
		require.True(t, ok, "phase %q missing from RPC response", code)
		for locale, title := range lt.ByLocale {
			assert.Equal(t, title, pbTitles.ByLocale[locale],
				"phase %q locale %q mismatch", code, locale)
		}
	}
	for code, lt := range catalog.SubPhases {
		pbTitles, ok := got.SubPhases[code]
		require.True(t, ok, "subphase %q missing from RPC response", code)
		for locale, title := range lt.ByLocale {
			assert.Equal(t, title, pbTitles.ByLocale[locale],
				"subphase %q locale %q mismatch", code, locale)
		}
	}
}
