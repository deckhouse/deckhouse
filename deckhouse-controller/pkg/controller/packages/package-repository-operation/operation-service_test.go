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

package packagerepositoryoperation

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestFilterLatestTags(t *testing.T) {
	tags := []*semver.Version{
		semver.MustParse("v1.0.0"),
		semver.MustParse("v1.0.1"),
		semver.MustParse("v1.0.2"),
		semver.MustParse("v1.1.0"),
		semver.MustParse("v1.1.1"),
		semver.MustParse("v3.3.3"),
		semver.MustParse("v4.0.0"),
	}

	result := filterLatestTags(tags)

	assert.Equal(t, len(result), 4)
	assert.Contains(t, result, semver.MustParse("v1.0.2"))
	assert.Contains(t, result, semver.MustParse("v1.1.1"))
	assert.Contains(t, result, semver.MustParse("v3.3.3"))
	assert.Contains(t, result, semver.MustParse("v4.0.0"))
}
