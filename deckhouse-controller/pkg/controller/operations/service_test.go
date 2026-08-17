// Copyright 2025 Flant JSC
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
	"sort"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestFilterLatestTags(t *testing.T) {
	// versions builds a slice of parsed semver versions from their string form.
	versions := func(raw ...string) []*semver.Version {
		out := make([]*semver.Version, 0, len(raw))
		for _, r := range raw {
			out = append(out, semver.MustParse(r))
		}
		return out
	}

	// toStrings converts the result to a sorted slice of version strings so it can be
	// compared regardless of the map-iteration order the function returns.
	toStrings := func(tags []*semver.Version) []string {
		out := make([]string, 0, len(tags))
		for _, tag := range tags {
			out = append(out, tag.String())
		}
		sort.Strings(out)
		return out
	}

	tests := []struct {
		name  string
		input []*semver.Version
		want  []string
	}{
		{
			name:  "empty input",
			input: nil,
			want:  []string{},
		},
		{
			name:  "only nil entries are skipped",
			input: []*semver.Version{nil, nil},
			want:  []string{},
		},
		{
			name:  "single version",
			input: versions("1.2.3"),
			want:  []string{"1.2.3"},
		},
		{
			name:  "keeps the latest patch within a major.minor",
			input: versions("1.0.0", "1.0.5", "1.0.2"),
			want:  []string{"1.0.5"},
		},
		{
			name:  "keeps the latest patch per minor within a major",
			input: versions("1.0.0", "1.0.9", "1.1.0", "1.1.3", "1.2.1"),
			want:  []string{"1.0.9", "1.1.3", "1.2.1"},
		},
		{
			name:  "keeps the latest patch per minor across majors",
			input: versions("1.0.0", "1.0.1", "2.0.0", "2.3.4", "2.3.1", "3.5.0"),
			want:  []string{"1.0.1", "2.0.0", "2.3.4", "3.5.0"},
		},
		{
			name:  "unordered input still selects the max patch",
			input: versions("1.4.2", "1.4.9", "1.4.0", "1.4.7"),
			want:  []string{"1.4.9"},
		},
		{
			name:  "duplicate versions collapse to one",
			input: versions("1.0.0", "1.0.0", "1.0.0"),
			want:  []string{"1.0.0"},
		},
		{
			name:  "nil entries mixed with real versions are skipped",
			input: []*semver.Version{semver.MustParse("2.1.0"), nil, semver.MustParse("2.1.5")},
			want:  []string{"2.1.5"},
		},
		{
			name:  "release is preferred over its prerelease of the same version",
			input: versions("1.2.0-rc.1", "1.2.0"),
			want:  []string{"1.2.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterLatestTags(tt.input)
			assert.ElementsMatch(t, tt.want, toStrings(got))
		})
	}
}
