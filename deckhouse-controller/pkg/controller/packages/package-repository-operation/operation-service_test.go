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
