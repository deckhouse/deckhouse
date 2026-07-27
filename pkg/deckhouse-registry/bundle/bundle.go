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

// Package bundle addresses the repositories that hold artifact bundles.
//
// A bundle is a full image — one shipping the artifact itself, as opposed to a
// release image (metadata only) or a catalog entry (scratch). Five repositories
// hold them:
//
//	<root>/<edition>                        the Deckhouse image
//	<root>/<edition>/install                the installer image
//	<root>/<edition>/install-standalone     the standalone installer image
//	<root>/<edition>/modules/<module>       a module image
//	<root>/<edition>/packages/<package>     a package image
//	<root>/installer                        the edition-independent installer
//
// What they have in common, and what this package adds, is images_digests.json:
// the map from every image the bundle contains to its content-addressable
// digest.
package bundle

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/digests"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// DigestsPath is where a bundle keeps images_digests.json, relative to the
// image root.
const DigestsPath = digests.FileName

// Service is a repository holding artifact bundles.
type Service struct {
	*service.BasicService
}

// New wraps a repository service as a bundle service.
func New(svc *service.BasicService) *Service {
	return &Service{BasicService: svc}
}

// Digests reads images_digests.json out of the bundle image at tag, mapping
// each image the bundle ships to its content-addressable digest.
//
// The Deckhouse image bundles every module of its edition, so its file is keyed
// by module and the result is nested; a module, package or installer image
// bundles only its own images and the result is flat. digests.Digests.IsNested
// says which, so the same call handles both.
//
// Reading a bundle means pulling and flattening a full image, which for the
// Deckhouse image is hundreds of megabytes.
func (s *Service) Digests(ctx context.Context, tag string) (*digests.Digests, error) {
	entry := s.Entry(tag)

	entry.Debug("Getting image digests", slog.String("file", DigestsPath))

	img, err := s.GetImage(ctx, tag)
	if err != nil {
		return nil, err
	}

	rc := img.Extract()
	defer rc.Close()

	parsed, err := digests.Read(rc, DigestsPath)
	if err != nil {
		return nil, fmt.Errorf("digests of %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Image digests retrieved",
		slog.Bool("nested", parsed.IsNested()),
		slog.Int("images", parsed.Count()),
	)

	return parsed, nil
}
