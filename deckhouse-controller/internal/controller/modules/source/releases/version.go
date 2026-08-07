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

package releases

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// versionTag matches a plain "vX.Y.Z" registry tag; anything else - a channel name, a dev tag, a
// prerelease - is not a version this package will build a release from.
var versionTag = regexp.MustCompile(`^v(([0-9]+).([0-9]+).([0-9]+))$`)

// minVersionsCapacity is the usual length of one walk, used to size the result up front.
const minVersionsCapacity = 10

// newVersions returns the highest patch of every minor between actual and target, inclusive of the
// current minor's patches, which is what stops the walk skipping a migration.
//
// For a registry holding 1.66.3 (deployed), 1.66.5, 1.67.5, 1.67.11, 1.68.1, 1.68.3 and 1.68.5,
// the walk is [1.66.5, 1.67.11, 1.68.5].
func (e *ensurer) newVersions(ctx context.Context, actual, target *semver.Version) ([]*semver.Version, error) {
	tags, err := e.registry.ListTags(ctx, e.req.Remote, e.req.ModuleName)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	collection := e.parseVersions(tags)
	if len(collection) == 0 {
		return nil, fmt.Errorf("no matched tags in registry")
	}

	sort.Sort(semver.Collection(collection))

	result := make([]*semver.Version, 0, minVersionsCapacity)

	var last *semver.Version
	for _, version := range collection {
		if !inRange(version, actual, target) {
			continue
		}

		// the previous version was the highest patch of its minor
		if last != nil && (last.Minor() < version.Minor() || last.Major() < version.Major()) {
			result = append(result, last)
		}

		last = version
	}

	if last == nil {
		// an empty result is not an error - actual is already at or past the target
		return result, nil
	}

	if greaterThanTarget(last, target) {
		e.logger.Warn("last release is not equal to the target, using the target instead",
			slog.String("last", last.Original()),
			slog.String("target", target.Original()))

		return append(result, target), nil
	}

	return append(result, last), nil
}

// parseVersions keeps the registry tags that are plain versions, skipping everything else.
func (e *ensurer) parseVersions(tags []string) []*semver.Version {
	versions := make([]*semver.Version, 0, len(tags))

	for _, tag := range tags {
		if !versionTag.MatchString(tag) {
			e.logger.Debug("the tag is not a version, skip it", slog.String("version", tag))
			continue
		}

		version, err := semver.NewVersion(tag)
		if err != nil {
			e.logger.Warn("failed to parse the version from the registry, skip it", slog.String("version", tag))
			continue
		}

		versions = append(versions, version)
	}

	return versions
}

// inRange reports whether the version sits between actual and target: strictly above actual, and no
// further than the target's minor.
//
//	[actual=v1.4.1, ver=v1.4.0, target=v1.5.2] -> false  // at or below actual
//	[actual=v1.4.1, ver=v1.4.4, target=v1.5.2] -> true   // a patch of the current minor
//	[actual=v1.4.1, ver=v1.5.2, target=v1.5.2] -> true   // the target itself
//	[actual=v1.4.1, ver=v1.6.0, target=v1.5.2] -> false  // past the target's minor
func inRange(version, actual, target *semver.Version) bool {
	if !version.GreaterThan(actual) {
		return false
	}

	return version.Major() < target.Major() ||
		(version.Major() == target.Major() && version.Minor() <= target.Minor())
}

// greaterThanTarget reports whether the version overshoots the target.
func greaterThanTarget(version, target *semver.Version) bool {
	return version.Major() > target.Major() ||
		(version.Major() == target.Major() && version.Minor() > target.Minor()) ||
		(version.Major() == target.Major() && version.Minor() == target.Minor() && version.Patch() > target.Patch())
}
