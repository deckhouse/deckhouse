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

// Package deckhouse models the Deckhouse platform images at the edition root:
//
//	<root>/<edition>:<version|channel>
//	<root>/<edition>/release-channel:<channel|version>
//	<root>/<edition>/install:<version|channel>
//	<root>/<edition>/install-standalone:<version|channel>
package deckhouse

import (
	"context"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Fixed path segments of the Deckhouse sub-tree.
const (
	// ReleaseChannelSegment holds Deckhouse release images.
	ReleaseChannelSegment = "release-channel"
	// InstallSegment holds the edition-scoped installer image.
	InstallSegment = "install"
	// InstallStandaloneSegment holds the standalone installer image.
	InstallStandaloneSegment = "install-standalone"
)

// Service names used in log records.
const (
	ServiceName           = "deckhouse"
	releaseServiceName    = "deckhouse_release"
	installServiceName    = "deckhouse_install"
	standaloneServiceName = "deckhouse_install_standalone"
)

// Service addresses the Deckhouse platform. The service itself is the Deckhouse
// image repository at the edition root, and it carries the three repositories
// that hang directly off it.
// ReleaseService is the Deckhouse release repository
// (<root>/<edition>/release-channel).
//
// It differs from a module or package release in what the image carries: a
// Deckhouse release has no definition file, and instead fills in the rollout
// fields of version.json — Canary, Disruptions and Requirements — which drive
// how the platform upgrade is staged.
type ReleaseService struct {
	*release.Service
}

// Metadata returns the decoded version.json of the Deckhouse release image at
// tag, including the rollout controls that only a Deckhouse release carries.
func (s *ReleaseService) Metadata(ctx context.Context, tag string) (*release.DeckhouseVersion, error) {
	return s.DeckhouseVersion(ctx, tag)
}

type Service struct {
	*bundle.Service

	releases          *ReleaseService
	install           *bundle.Service
	installStandalone *bundle.Service
}

// New builds the Deckhouse sub-tree. editionRoot addresses the edition root
// itself, which is also the Deckhouse image repository.
//
// The Deckhouse image and both installers are artifact bundles and carry
// images_digests.json; the release-channel repository is not and does not.
func New(editionRoot *service.BasicService) *Service {
	return &Service{
		Service:           bundle.New(editionRoot),
		releases:          &ReleaseService{Service: release.New(editionRoot.Sub(releaseServiceName, ReleaseChannelSegment))},
		install:           bundle.New(editionRoot.Sub(installServiceName, InstallSegment)),
		installStandalone: bundle.New(editionRoot.Sub(standaloneServiceName, InstallStandaloneSegment)),
	}
}

// Releases returns the Deckhouse release-channel repository, whose tags are
// channel names and versions and whose images carry release metadata.
func (s *Service) Releases() *ReleaseService {
	return s.releases
}

// Install returns the edition-scoped Deckhouse installer image repository
// (<root>/<edition>/install), distinct from the edition-independent
// <root>/installer.
func (s *Service) Install() *bundle.Service {
	return s.install
}

// InstallStandalone returns the standalone installer image repository
// (<root>/<edition>/install-standalone).
func (s *Service) InstallStandalone() *bundle.Service {
	return s.installStandalone
}
