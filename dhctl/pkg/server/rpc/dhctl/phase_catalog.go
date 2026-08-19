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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

// GetPhaseCatalog returns the localized titles for every phase and subphase
// code this dhctl instance can emit. It shares the single i18n loader with
// the CLI progress bar and the `phase-catalog` CLI command, so all three
// transports cannot diverge.
func (s *Service) GetPhaseCatalog(_ context.Context, _ *pb.PhaseCatalogRequest) (*pb.PhaseCatalog, error) {
	titles, err := phases.LoadTitles()
	if err != nil {
		return nil, fmt.Errorf("loading phase titles: %w", err)
	}

	catalog := titles.ToCatalog()
	return &pb.PhaseCatalog{
		Phases:    toPBTitles(catalog.Phases),
		SubPhases: toPBTitles(catalog.SubPhases),
	}, nil
}

func toPBTitles(byCode map[string]phases.LocaleTitles) map[string]*pb.Titles {
	out := make(map[string]*pb.Titles, len(byCode))
	for code, lt := range byCode {
		out[code] = &pb.Titles{ByLocale: lt.ByLocale}
	}
	return out
}
