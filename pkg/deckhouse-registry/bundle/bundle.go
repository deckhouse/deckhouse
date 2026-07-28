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
// release image (metadata only) or a catalog entry (scratch). What they have in
// common, and what this package adds, is images_digests.json: the map from
// every image the bundle contains to its content-addressable digest.
//
// Where that file sits inside the image is not common, so each bundle declares
// its own path:
//
//	<root>/<edition>                      deckhouse/modules/…   nested
//	<root>/<edition>/install              deckhouse/candi/…     nested
//	<root>/<edition>/install-standalone   deckhouse/candi/…     nested
//	<root>/installer                      deckhouse/candi/…     nested
//	<root>/<edition>/modules/<module>     images_digests.json   flat
//	<root>/<edition>/packages/<package>   images_digests.json   flat
//
// Verified at v1.76.6: the Deckhouse image carries the file under
// deckhouse/modules/ and the install image under deckhouse/candi/ — neither at
// the image root — while a module image carries it at the root.
package bundle

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/digests"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Well-known locations of images_digests.json, relative to the image root.
const (
	// RootPath is where a module or package image keeps its digests.
	RootPath = digests.FileName
	// ModulesImagesDigestsPath is where the Deckhouse image keeps the digests of every
	// module it bundles. That file is nested.
	ModulesImagesDigestsPath = "deckhouse/modules/" + digests.FileName
	// CandiImagesDigestsPath is where the installer images keep theirs, also nested.
	CandiImagesDigestsPath = "deckhouse/candi/" + digests.FileName
)

// Service is a repository holding artifact bundles.
type Service struct {
	*service.BasicService

	digestsPath string
}

// New wraps a repository service as a bundle whose images_digests.json sits at
// digestsPath inside the image — one of the constants above.
func New(svc *service.BasicService, digestsPath string) *Service {
	return &Service{BasicService: svc, digestsPath: digestsPath}
}

// DigestsPath returns where this bundle keeps images_digests.json.
func (s *Service) DigestsPath() string {
	return s.digestsPath
}

// Fetch pulls the bundle image at tag once and returns a Bundle snapshot that
// serves its images_digests.json from memory.
//
// Reading a bundle means pulling and flattening a full image, which for the
// Deckhouse image is hundreds of megabytes — so pull once and read the result,
// rather than per field.
func (s *Service) Fetch(ctx context.Context, tag string) (*Bundle, error) {
	entry := s.Entry(tag)

	entry.Debug("Fetching bundle image", slog.String("file", s.digestsPath))

	img, err := s.GetImage(ctx, tag)
	if err != nil {
		return nil, err
	}

	rc := img.Extract()
	defer rc.Close()

	parsed, err := digests.Read(rc, s.digestsPath)
	if err != nil {
		return nil, fmt.Errorf("digests of %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Bundle image fetched",
		slog.Bool("nested", parsed.IsNested()),
		slog.Int("images", parsed.Count()),
	)

	return &Bundle{ref: s.Ref(tag), digests: parsed}, nil
}

// Bundle is a bundle image read once. Its accessors serve the extracted
// content from memory and never touch the registry.
type Bundle struct {
	ref     string
	digests *digests.Digests
}

// Ref is the fully-qualified reference the snapshot was read from.
func (b *Bundle) Ref() string {
	return b.ref
}

// Digests is the decoded images_digests.json — every image the bundle ships
// mapped to its content-addressable digest.
//
// The shape follows from what the bundle contains. The Deckhouse image and the
// installers carry the images of many modules, so their file is keyed by module
// and the result is nested; a module or package image carries only its own, so
// the result is flat. digests.Digests.IsNested says which.
func (b *Bundle) Digests() *digests.Digests {
	return b.digests
}
