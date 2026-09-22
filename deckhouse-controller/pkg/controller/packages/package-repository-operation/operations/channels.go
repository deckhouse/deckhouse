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

package operations

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	registryService "github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry/service"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// knownReleaseChannels is what makes a listed tag a channel: the /release path holds channel tags
// and version tags together, and nothing in a tag tells them apart.
var knownReleaseChannels = []string{
	"alpha",
	"beta",
	"early-access",
	"stable",
	"rock-solid",
	"lts",
}

// releaseChannelReader is a registry path whose images carry the version.json of a channel.
type releaseChannelReader interface {
	GetMetadata(ctx context.Context, tag string) (*registryService.PackageVersionMetadata, error)
}

// releaseChannelSource is where a package's channels were found. The zero value means none were, and
// carries no reader.
type releaseChannelSource struct {
	reader releaseChannelReader
	tags   []string
}

// ScanReleaseChannels reads the version behind every release channel the package offers. An empty map
// means it offers none; an error means they could not be read, and the caller keeps what it knows.
// A single unreadable channel is skipped rather than failing the rest.
func (s *OperationService) ScanReleaseChannels(ctx context.Context, packageName string) (map[string]string, error) {
	source, err := s.findReleaseChannels(ctx, packageName)
	if err != nil {
		return nil, err
	}

	var namedChannels int

	channels := make(map[string]string, len(knownReleaseChannels))
	for _, tag := range source.tags {
		if !slices.Contains(knownReleaseChannels, tag) {
			continue
		}

		namedChannels++

		meta, err := source.reader.GetMetadata(ctx, tag)
		if err != nil {
			s.logger.Warn(
				"failed to read the version a release channel points to",
				slog.String("package", packageName),
				slog.String("channel", tag),
				log.Err(err),
			)

			continue
		}

		if meta.Version == "" {
			s.logger.Warn(
				"release channel image carries no version",
				slog.String("package", packageName),
				slog.String("channel", tag),
			)

			continue
		}

		channels[tag] = meta.Version
	}

	// Nothing read though channels exist: an empty result would read as "offered on no channel" and
	// drop the matrix.
	if namedChannels > 0 && len(channels) == 0 {
		return nil, fmt.Errorf("read none of the %d release channels of package %q", namedChannels, packageName)
	}

	s.logger.Debug(
		"scanned release channels",
		slog.String("package", packageName),
		slog.Int("count", len(channels)),
	)

	return channels, nil
}

// findReleaseChannels locates the path a package's channels were published to: <package>/release-channel,
// falling back to <package>/release, which mixes channel tags with version tags.
//
// An empty listing falls back like a missing path: a registry may answer a path it does not hold with
// an empty tag list instead of NAME_UNKNOWN.
func (s *OperationService) findReleaseChannels(ctx context.Context, packageName string) (releaseChannelSource, error) {
	pkg := s.svc.Package(packageName)

	channels := pkg.ReleaseChannels()

	tags, err := channels.ListTags(ctx)
	if err != nil && !isRepoNotFoundError(err) {
		return releaseChannelSource{}, fmt.Errorf("list release channel tags: %w", err)
	}
	if len(tags) > 0 {
		return releaseChannelSource{reader: channels, tags: tags}, nil
	}

	release := pkg.Release()

	tags, err = release.ListTags(ctx)
	if err != nil {
		if isRepoNotFoundError(err) {
			return releaseChannelSource{}, nil
		}

		return releaseChannelSource{}, fmt.Errorf("list release tags: %w", err)
	}

	return releaseChannelSource{reader: release, tags: tags}, nil
}
