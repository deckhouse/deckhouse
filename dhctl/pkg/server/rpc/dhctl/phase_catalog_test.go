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

func TestGetPhaseCatalog(t *testing.T) {
	t.Parallel()

	s := New(ServiceParams{})
	resp, err := s.GetPhaseCatalog(context.Background(), &pb.PhaseCatalogRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Both namespaces present and non-empty.
	require.NotEmpty(t, resp.Phases)
	require.NotEmpty(t, resp.SubPhases)

	// Every supported language must be populated for a known phase code.
	base := resp.Phases[string(phases.BaseInfraPhase)]
	require.NotNil(t, base)
	for _, lang := range phases.Languages {
		_, ok := base.ByLocale[string(lang)]
		assert.True(t, ok, "BaseInfra missing %q locale", lang)
	}
	assert.Equal(t, "Base Infrastructure", base.ByLocale["en"])

	// Overlapping code must resolve to distinct titles per namespace — the
	// reason two maps exist.
	phaseTitle := resp.Phases[string(phases.InstallDeckhousePhase)].ByLocale["en"]
	subTitle := resp.SubPhases[string(phases.InstallDeckhousePhase)].ByLocale["en"]
	assert.Equal(t, "Install Deckhouse", phaseTitle)
	assert.Equal(t, "Install...", subTitle)
}

// TestGetPhaseCatalog_MatchesLoader asserts the RPC and the shared loader
// return the same content — the two transports (gRPC and CLI) cannot diverge
// because they read one loader.
func TestGetPhaseCatalog_MatchesLoader(t *testing.T) {
	t.Parallel()

	titles, err := phases.LoadTitles()
	require.NoError(t, err)

	catalog := titles.ToCatalog()
	require.NotEmpty(t, catalog.Phases)
	require.NotEmpty(t, catalog.SubPhases)

	s := New(ServiceParams{})
	resp, err := s.GetPhaseCatalog(context.Background(), &pb.PhaseCatalogRequest{})
	require.NoError(t, err)

	for code, byLocale := range catalog.Phases {
		pbTitles, ok := resp.Phases[code]
		require.True(t, ok, "phase %q missing from RPC response", code)
		for locale, title := range byLocale {
			assert.Equal(t, title, pbTitles.ByLocale[locale],
				"phase %q locale %q mismatch", code, locale)
		}
	}
	for code, byLocale := range catalog.SubPhases {
		pbTitles, ok := resp.SubPhases[code]
		require.True(t, ok, "subphase %q missing from RPC response", code)
		for locale, title := range byLocale {
			assert.Equal(t, title, pbTitles.ByLocale[locale],
				"subphase %q locale %q mismatch", code, locale)
		}
	}
}
