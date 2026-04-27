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

package source

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestIsVersionInRange(t *testing.T) {
	tests := []struct {
		name     string
		ver      string
		actual   string
		target   string
		expected bool
	}{
		{"ver is lower patch of current minor", "1.4.0", "1.4.1", "1.5.2", false},
		{"ver equals actual", "1.4.1", "1.4.1", "1.5.2", false},
		{"ver is higher patch of current minor", "1.4.4", "1.4.1", "1.5.2", true},
		{"ver is the target version", "1.5.2", "1.4.1", "1.5.2", true},
		{"ver is between actual and target", "1.5.0", "1.4.1", "1.6.0", true},
		{"ver exceeds target minor", "1.6.0", "1.4.1", "1.5.2", false},
		{"ver is next major but within target", "2.0.1", "1.33.0", "2.1.0", true},
		{"ver is lower major", "0.9.0", "1.0.0", "1.2.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isVersionInRange(
				semver.MustParse(tt.ver),
				semver.MustParse(tt.actual),
				semver.MustParse(tt.target),
			)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetNewVersions(t *testing.T) {
	registryTags := []string{
		"v1.31.0",
		"v1.31.1",
		"v1.32.0",
		"v1.32.1",
		"v1.32.2",
		"v1.32.3",
		"v1.33.0",
		"v1.33.1",
		"v2.0.0",
		"v2.0.1",
		"v2.0.5",
		"v2.1.2",
		"v2.1.12",
		"v2.5.1",
	}

	check := func(name string, actualStr, targetStr string, expected []*semver.Version) {
		t.Run(name, func(t *testing.T) {
			mockClient := cr.NewClientMock(t)
			mockClient.ListTagsMock.Return(registryTags, nil)

			f := &ModuleReleaseFetcher{
				registryClientTagFetcher: mockClient,
				logger:                   log.NewNop(),
			}

			actual, err := semver.NewVersion(actualStr)
			require.NoError(t, err)
			target, err := semver.NewVersion(targetStr)
			require.NoError(t, err)

			got, err := f.getNewVersions(context.Background(), actual, target)
			require.NoError(t, err)

			if !cmp.Equal(got, expected) {
				t.Fatalf("got != expected, diff:\n%s", cmp.Diff(got, expected))
			}
		})
	}

	check("patch of current minor", "1.31.0", "1.31.1",
		[]*semver.Version{semver.MustParse("1.31.1")})

	check("patch of current minor is kept on minor bump", "1.31.0", "1.32.3",
		[]*semver.Version{
			semver.MustParse("1.31.1"),
			semver.MustParse("1.32.3"),
		})

	check("several minors", "1.31.0", "1.33.1",
		[]*semver.Version{
			semver.MustParse("1.31.1"),
			semver.MustParse("1.32.3"),
			semver.MustParse("1.33.1"),
		})

	check("major bump", "1.31.0", "2.0.5",
		[]*semver.Version{
			semver.MustParse("1.31.1"),
			semver.MustParse("1.32.3"),
			semver.MustParse("1.33.1"),
			semver.MustParse("2.0.5"),
		})

	check("across major and minor", "1.31.0", "2.1.12",
		[]*semver.Version{
			semver.MustParse("1.31.1"),
			semver.MustParse("1.32.3"),
			semver.MustParse("1.33.1"),
			semver.MustParse("2.0.5"),
			semver.MustParse("2.1.12"),
		})

	check("last minor has no patches matching target", "1.31.0", "1.33.0",
		[]*semver.Version{
			semver.MustParse("1.31.1"),
			semver.MustParse("1.32.3"),
			semver.MustParse("1.33.0"),
		})

	check("actual equals target returns empty", "1.31.1", "1.31.1",
		[]*semver.Version{})

	check("actual above target returns empty", "1.33.1", "1.32.3",
		[]*semver.Version{})
}
