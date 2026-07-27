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
// Every sub-tree publishes releases, and every release image carries
// version.json, so that much is shared here. What sits alongside it is not:
// a module release ships module.yaml, a package release ships package.yaml,
// and a Deckhouse release ships neither but fills in the rollout fields of
// version.json (canary, disruptions, requirements). The getters for those live
// on the sub-tree types that own them — module.ReleaseService.Definition and
// packages.VersionService.Definition.
//
// Every file this package knows has a mapping, so the getters return decoded
// values, never bytes. Each decoded result keeps the undecoded original where
// a consumer needs its own schema, and File reads anything not mapped here.
//
// The segment a release repository hangs off differs too: release-channel for
// Deckhouse, release for modules, version for packages and the CLI.
package release

import (
	"context"
	"errors"
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

// Files pulls the release image at tag and returns the named metadata files.
// Names are relative to the image root. A file the image does not carry is
// simply absent from the result.
//
// Every getter below is a thin wrapper over this. Prefer calling it directly
// when you need more than one file: each getter pulls the image again, and one
// call reads them all in a single pass.
func (s *Service) Files(ctx context.Context, tag string, names ...string) (map[string][]byte, error) {
	entry := s.Entry(tag)

	entry.Debug("Reading release image files", slog.Any("files", names))

	img, err := s.GetImage(ctx, tag)
	if err != nil {
		return nil, err
	}

	rc := img.Extract()
	defer rc.Close()

	files, err := Read(rc, names...)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Ref(tag), err)
	}

	entry.Debug("Release image files read", slog.Int("found", len(files)))

	return files, nil
}

// File pulls the release image at tag and returns one metadata file, or
// ErrFileNotFound when the image does not carry it.
func (s *Service) File(ctx context.Context, tag, name string) ([]byte, error) {
	files, err := s.Files(ctx, tag, name)
	if err != nil {
		return nil, err
	}

	raw, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s has no %s", ErrFileNotFound, s.Ref(tag), name)
	}

	return raw, nil
}

// versionJSON reads the raw version.json of the release image at tag. It stays
// unexported: version.json has a mapping for each kind of release, so callers
// get a decoded result rather than bytes. Raw is still on the result for
// consumers applying their own schema.
func (s *Service) versionJSON(ctx context.Context, tag string) ([]byte, error) {
	raw, err := s.File(ctx, tag, VersionFile)
	if err != nil {
		if errors.Is(err, ErrFileNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNoVersionMetadata, s.Ref(tag))
		}

		return nil, err
	}

	return raw, nil
}

// DeckhouseVersion reads and decodes version.json under the Deckhouse schema,
// which carries the rollout controls. Reach it through
// deckhouse.ReleaseService.Metadata, the only release repository it applies to.
func (s *Service) DeckhouseVersion(ctx context.Context, tag string) (*DeckhouseVersion, error) {
	raw, err := s.versionJSON(ctx, tag)
	if err != nil {
		return nil, err
	}

	version, err := ParseDeckhouseVersion(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	return version, nil
}

// PackageVersion reads and decodes version.json under the module and package
// schema, which declares only the version. Reach it through
// module.ReleaseService.Metadata or packages.VersionService.Metadata.
func (s *Service) PackageVersion(ctx context.Context, tag string) (*PackageVersion, error) {
	raw, err := s.versionJSON(ctx, tag)
	if err != nil {
		return nil, err
	}

	version, err := ParsePackageVersion(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	return version, nil
}

// Version returns the version a release image declares — the common case,
// resolving a channel name to a concrete version.
//
// It reads only the version field, the one thing both version.json schemas
// share, so it works on any release repository. For the rest, use the Metadata
// of the sub-tree service, which knows which schema applies.
func (s *Service) Version(ctx context.Context, tag string) (string, error) {
	raw, err := s.versionJSON(ctx, tag)
	if err != nil {
		return "", err
	}

	version, err := parseCommonVersion(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	if version == "" {
		return "", fmt.Errorf("%w: %s declares no version", ErrNoVersionMetadata, s.Ref(tag))
	}

	s.Entry(tag).Debug("Release version resolved", slog.String("version", version))

	return version, nil
}

// changelogYAML reads the raw changelog of the release image at tag, trying
// both spellings the build emits. It stays unexported: the changelog has a
// mapping, so callers get a decoded result rather than bytes.
func (s *Service) changelogYAML(ctx context.Context, tag string) ([]byte, error) {
	files, err := s.Files(ctx, tag, ChangelogFile, ChangelogFileAlt)
	if err != nil {
		return nil, err
	}

	for _, name := range []string{ChangelogFile, ChangelogFileAlt} {
		if raw, ok := files[name]; ok {
			return raw, nil
		}
	}

	return nil, fmt.Errorf("%w: %s has no changelog", ErrFileNotFound, s.Ref(tag))
}

// Changelog returns the decoded changelog of the release image at tag.
//
// A changelog that fails to parse is reported as an error here, but callers
// driving a rollout should treat it as non-fatal: a broken changelog never
// blocks a release.
func (s *Service) Changelog(ctx context.Context, tag string) (map[string]any, error) {
	raw, err := s.changelogYAML(ctx, tag)
	if err != nil {
		return nil, err
	}

	changelog, err := ParseChangelog(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Ref(tag), err)
	}

	return changelog, nil
}

// Channels returns the release channel names published in this repository —
// the tags that are not concrete versions or digests.
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
