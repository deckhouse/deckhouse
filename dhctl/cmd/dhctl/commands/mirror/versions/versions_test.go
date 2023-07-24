// Copyright 2023 Flant JSC
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

package versions

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseFromInt(t *testing.T) {
	type args struct {
		major uint64
		minor uint64
		patch uint64
	}
	tests := []struct {
		name string
		args args
		want *semver.Version
	}{
		{
			name: "test major minor",
			args: args{
				major: 1,
				minor: 34,
			},
			want: semver.MustParse("v1.34"),
		},
		{
			name: "test major minor patch",
			args: args{
				major: 1,
				minor: 34,
				patch: 123,
			},
			want: semver.MustParse("v1.34.123"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFromInt(tt.args.major, tt.args.minor, tt.args.patch)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestLatestVersionSetGet(t *testing.T) {
	latestVersions := make(latestVersions)
	_, err := latestVersions.GetString("v1.34")
	require.ErrorIs(t, err, ErrNoVersion)

	_, err = latestVersions.Get(*semver.MustParse("v1.46"))
	require.ErrorIs(t, err, ErrNoVersion)

	ok, err := latestVersions.SetString("v1.46.4")
	assert.NoError(t, err)
	assert.True(t, ok)

	v, err := latestVersions.GetString("v1.46.10")
	assert.NoError(t, err)
	assert.Equal(t, v, semver.MustParse("v1.46.4"))

	v, err = latestVersions.Get(*semver.MustParse("v1.46"))
	assert.NoError(t, err)
	assert.Equal(t, v, semver.MustParse("v1.46.4"))

	ok, err = latestVersions.Set(*semver.MustParse("v1.46.3"))
	assert.NoError(t, err)
	assert.False(t, ok)

	v, err = latestVersions.GetString("v1.46.10")
	assert.NoError(t, err)
	assert.Equal(t, v, semver.MustParse("v1.46.4"))

	ok, err = latestVersions.Set(*semver.MustParse("v1.46.10"))
	assert.NoError(t, err)
	assert.True(t, ok)

	v, err = latestVersions.GetString("v1.46.124")
	assert.NoError(t, err)
	assert.Equal(t, v, semver.MustParse("v1.46.10"))
}

func TestLatestVersionOldestLatest(t *testing.T) {
	latestVersions := make(latestVersions)
	for _, v := range []string{"v1.34.10", "v1.45.10", "v1.50"} {
		ok, err := latestVersions.SetString(v)
		require.NoError(t, err)
		require.True(t, ok)
	}

	latest := latestVersions.Latest()
	assert.Equal(t, latest, semver.MustParse("v1.50"))

	oldest := latestVersions.Oldest()
	assert.Equal(t, oldest, semver.MustParse("v1.34.10"))
}
