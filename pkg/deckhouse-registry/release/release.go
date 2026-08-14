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

// Package release reads release images — the lightweight scratch images that
// carry only the metadata of the artifact they describe.
//
// A release image is pulled once, by Service.Fetch, into a Release snapshot;
// every field — version, changelog, the raw files — is then served from memory.
// Nothing re-downloads, so reading five fields costs one pull, not five.
//
// version.json is common to every release. What sits alongside it is not: a
// module release ships module.yaml, a package release ships package.yaml, and a
// Deckhouse release ships neither but fills in the rollout fields of
// version.json (canary, disruptions, requirements). Which version.json schema
// applies and which manifest is present is fixed by the sub-tree that owns the
// release, so the typed accessors live on its snapshot — deckhouse.Release,
// module.Release, packages.Release — each wrapping the Release returned here.
package release

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// Service is a repository holding release images.
//
// Its tags are either channel names (alpha, beta, early-access, stable,
// rock-solid, lts) or concrete versions (v1.73.0), so the same service answers
// both "what is on the stable channel" and "what does v1.73.0 declare".
type Service struct {
	*service.BasicService
}

// New wraps a repository service as a release service.
func New(svc *service.BasicService) *Service {
	return &Service{BasicService: svc}
}

// Fetch pulls the release image at tag and extracts its files once. The
// returned Release serves version, changelog and the raw files from memory —
// read as many as you need without pulling again.
func (s *Service) Fetch(ctx context.Context, tag string) (*Release, error) {
	entry := s.Entry(tag)

	entry.Debug("Fetching release image")

	img, err := s.GetImage(ctx, tag)
	if err != nil {
		return nil, err
	}

	rc := img.Extract()
	defer rc.Close()

	files, err := readAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Release image fetched", slog.Int("files", len(files)))

	return &Release{ref: s.Ref(tag), files: files}, nil
}

// Channels returns the release channel names published in this repository —
// the tags that are not concrete versions or digests. It lists tags and pulls
// no image.
func (s *Service) Channels(ctx context.Context) ([]string, error) {
	tags, err := s.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	channels := make([]string, 0, len(tags))

	for _, tag := range tags {
		if IsChannel(tag) {
			channels = append(channels, tag)
		}
	}

	return channels, nil
}

// Release is a release image read once. Its accessors serve the extracted
// metadata from memory and never touch the registry.
//
// Both version.json decoders and the raw-file accessor are here; the sub-tree
// snapshots (deckhouse.Release and friends) pick the decoder that matches their
// kind and hide the rest.
type Release struct {
	ref   string
	files map[string][]byte
}

// Ref is the fully-qualified reference the snapshot was read from.
func (r *Release) Ref() string {
	return r.ref
}

// File returns a raw file from the image and whether it was present. Names are
// matched after the same normalization applied to the tar entries.
func (r *Release) File(name string) ([]byte, bool) {
	raw, ok := r.files[normalize(name)]

	return raw, ok
}

// VersionJSON returns the raw version.json, or ErrNoVersionMetadata when the
// image carries none.
func (r *Release) VersionJSON() ([]byte, error) {
	raw, ok := r.File(VersionFile)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoVersionMetadata, r.ref)
	}

	return raw, nil
}

// Version returns the version the release declares — the one field both
// version.json schemas share, so it works on any release.
func (r *Release) Version() (string, error) {
	raw, err := r.VersionJSON()
	if err != nil {
		return "", err
	}

	version, err := parseCommonVersion(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", r.ref, err)
	}

	if version == "" {
		return "", fmt.Errorf("%w: %s declares no version", ErrNoVersionMetadata, r.ref)
	}

	return version, nil
}

// DeckhouseVersion decodes version.json under the Deckhouse schema, which
// carries the rollout controls. Reach it through deckhouse.Release.Metadata.
func (r *Release) DeckhouseVersion() (*DeckhouseVersion, error) {
	raw, err := r.VersionJSON()
	if err != nil {
		return nil, err
	}

	version, err := ParseDeckhouseVersion(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.ref, err)
	}

	return version, nil
}

// PackageVersion decodes version.json under the module and package schema,
// which declares only the version. Reach it through module.Release.Metadata or
// packages.Release.Metadata.
func (r *Release) PackageVersion() (*PackageVersion, error) {
	raw, err := r.VersionJSON()
	if err != nil {
		return nil, err
	}

	version, err := ParsePackageVersion(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.ref, err)
	}

	return version, nil
}

// ChangelogYAML returns the raw changelog, trying both spellings the build
// emits, and whether one was present.
func (r *Release) ChangelogYAML() ([]byte, bool) {
	for _, name := range []string{ChangelogFile, ChangelogFileAlt} {
		if raw, ok := r.File(name); ok {
			return raw, true
		}
	}

	return nil, false
}

// Changelog returns the decoded changelog, or ErrFileNotFound when the image
// carries none.
//
// A changelog that fails to parse is an error here, but callers driving a
// rollout should treat it as non-fatal: a broken changelog never blocks a
// release.
func (r *Release) Changelog() (map[string]any, error) {
	raw, ok := r.ChangelogYAML()
	if !ok {
		return nil, fmt.Errorf("%w: %s has no changelog", ErrFileNotFound, r.ref)
	}

	changelog, err := ParseChangelog(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.ref, err)
	}

	return changelog, nil
}
