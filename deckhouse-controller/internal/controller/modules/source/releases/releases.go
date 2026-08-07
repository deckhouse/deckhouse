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

// Package releases owns the ModuleRelease objects of the modules a ModuleSource offers. It reports
// whether a module's update chain is complete and creates the releases missing from it, applying
// one sequencing rule to both so the two can never disagree about what counts as a gap.
package releases

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"

	"github.com/jonboulle/clockwork"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// tracerName names this package's spans.
const tracerName = "module-source-releases"

// Metadata is what one published module version says about itself.
type Metadata struct {
	// Version is the published version, "v"-prefixed as it appears in the registry.
	Version string
	// Checksum is the release image digest, which is what makes a fetch skippable.
	Checksum string
	// Changelog is the release's changelog, copied onto the ModuleRelease verbatim.
	Changelog map[string]any
	// Definition is the module.yaml the version ships; nil when the image carries none.
	Definition *moduletypes.Definition
}

// MetadataLoader loads the metadata a module published for one version. It is deliberately narrow:
// this package must not know how the bytes are fetched, so the registry client and the on-disk
// downloader stay on the caller's side of the boundary.
type MetadataLoader interface {
	ReleaseMetadata(ctx context.Context, moduleName, version string) (*Metadata, error)
}

// TagLister lists the versions published for a package path in a registry.
type TagLister interface {
	ListTags(ctx context.Context, remote registry.Remote, path ...string) ([]string, error)
}

// Config carries the collaborators the service keeps for every module it serves.
type Config struct {
	Client        client.Client
	Registry      TagLister
	Loader        MetadataLoader
	Clock         clockwork.Clock
	MetricStorage metricsstorage.Storage
	Logger        *log.Logger
}

// Service creates and inspects the ModuleReleases of the modules a source offers.
type Service struct {
	client        client.Client
	registry      TagLister
	loader        MetadataLoader
	clock         clockwork.Clock
	metricStorage metricsstorage.Storage
	logger        *log.Logger
}

// New builds a service shared by every module of every source.
func New(cfg Config) *Service {
	return &Service{
		client:        cfg.Client,
		registry:      cfg.Registry,
		loader:        cfg.Loader,
		clock:         cfg.Clock,
		metricStorage: cfg.MetricStorage,
		logger:        cfg.Logger,
	}
}

// Request describes one module's release fetch.
type Request struct {
	// Source owns the releases this fetch creates.
	Source *v1alpha1.ModuleSource
	// Remote is the registry the module's versions are published to.
	Remote registry.Remote
	// ModuleName is the module whose chain is being completed.
	ModuleName string
	// Target is the metadata of the version the release channel currently points at.
	Target *Metadata
	// UpdatePolicy names the policy that picked the release channel.
	UpdatePolicy string
	// ReleaseChannel is the channel the target came from; an LTS one skips the step-by-step walk.
	ReleaseChannel string
	// MetricGroup is the grouped-metric bucket to report this module's update failures in.
	MetricGroup string
}

// Exists reports whether a release with the given content checksum is already in the cluster. The
// digest is hashed first: it is 64 characters and a label value caps at 63.
func (s *Service) Exists(ctx context.Context, moduleName, checksum string) (bool, error) {
	hashed := fmt.Sprintf("%x", md5.Sum([]byte(checksum)))

	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.client.List(ctx, releases, client.MatchingLabels{
		v1alpha1.ModuleReleaseLabelModule:          moduleName,
		v1alpha1.ModuleReleaseLabelReleaseChecksum: hashed,
	}); err != nil {
		return false, fmt.Errorf("list module releases: %w", err)
	}

	s.logger.Debug("looked up the module release by checksum",
		slog.String("module_name", moduleName),
		slog.String("checksum", hashed),
		slog.Bool("exists", len(releases.Items) > 0))

	return len(releases.Items) > 0, nil
}

// list returns the module's releases.
func (s *Service) list(ctx context.Context, moduleName string) ([]*v1alpha1.ModuleRelease, error) {
	releases := new(v1alpha1.ModuleReleaseList)
	if err := s.client.List(ctx, releases, client.MatchingLabels{v1alpha1.ModuleReleaseLabelModule: moduleName}); err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	result := make([]*v1alpha1.ModuleRelease, 0, len(releases.Items))
	for i := range releases.Items {
		result = append(result, &releases.Items[i])
	}

	return result, nil
}
