/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeepRefusesAnUnknownDeployedVersion is the difference between "keep nothing" and "we
// do not know what to keep", which are opposite instructions. An empty keep-set against a
// store full of versions would delete every one of them.
func TestKeepRefusesAnUnknownDeployedVersion(t *testing.T) {
	for _, deployed := range []string{"", "   "} {
		_, err := Releases{Deployed: deployed, Previous: "v1.75.0"}.Keep()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nothing to keep against")
	}
}

func TestKeepWithoutAPreviousRelease(t *testing.T) {
	// A cluster that has never updated. One version to keep, and that is not an error.
	keep, err := Releases{Deployed: "v1.76.6"}.Keep()
	require.NoError(t, err)
	assert.Len(t, keep, 1)
	assert.Contains(t, keep, "v1.76.6")
}

// TestKeepNormalizesTheLeadingV: the registry holds both spellings depending on who pushed,
// and they are the same version.
func TestKeepNormalizesTheLeadingV(t *testing.T) {
	keep, err := Releases{Deployed: "1.76.6", Previous: "v1.75.0"}.Keep()
	require.NoError(t, err)
	assert.Contains(t, keep, "v1.76.6")
	assert.Contains(t, keep, "v1.75.0")
}

func TestJudge(t *testing.T) {
	releases := Releases{Deployed: "v1.76.6", Previous: "v1.75.0"}
	keep, err := releases.Keep()
	require.NoError(t, err)

	cases := []struct {
		name   string
		tag    string
		delete bool
		reason string
	}{{
		name:   "the deployed release",
		tag:    "v1.76.6",
		reason: ReasonCurrentRelease,
	}, {
		name:   "the previous release, kept so a rollback does not re-download it",
		tag:    "v1.75.0",
		reason: ReasonPreviousRelease,
	}, {
		// What the whole feature is for.
		name:   "a release the cluster has moved past",
		tag:    "v1.74.3",
		delete: true,
		reason: ReasonSuperseded,
	}, {
		name:   "the same version without the v",
		tag:    "1.76.6",
		reason: ReasonCurrentRelease,
	}, {
		// An update in flight. Deleting it makes the update re-download what it had
		// already fetched, and in air-gap deletes a release that was pushed on purpose and
		// cannot be fetched again.
		name:   "a release newer than the deployed one",
		tag:    "v1.77.0",
		reason: ReasonNewerRelease,
	}, {
		name:   "a much older release",
		tag:    "v1.40.0",
		delete: true,
		reason: ReasonSuperseded,
	}, {
		// Release channel names. Not versions, so what they point at is unknown, so they
		// stay.
		name:   "a release channel tag",
		tag:    "stable",
		reason: ReasonNotAVersion,
	}, {
		name:   "another release channel tag",
		tag:    "early-access",
		reason: ReasonNotAVersion,
	}, {
		name:   "a floating tag",
		tag:    "latest",
		reason: ReasonNotAVersion,
	}, {
		// Something an operator pushed by hand. This package has no idea what it is, which
		// is exactly why it does not touch it.
		name:   "a tag nobody can interpret",
		tag:    "my-debug-build",
		reason: ReasonNotAVersion,
	}, {
		name:   "a version with a build suffix, older",
		tag:    "v1.74.0-rc.1",
		delete: true,
		reason: ReasonSuperseded,
	}, {
		name:   "a version with a build suffix, newer",
		tag:    "v1.99.0-rc.1",
		reason: ReasonNewerRelease,
	}, {
		// Not a three-part version, so not something this package recognises.
		name:   "a two-part version",
		tag:    "v1.76",
		reason: ReasonNotAVersion,
	}, {
		name:   "a digest-looking tag",
		tag:    "sha256-abc123",
		reason: ReasonNotAVersion,
	}, {
		name:   "an empty tag",
		tag:    "",
		reason: ReasonNotAVersion,
	}, {
		name:   "whitespace around a version still counts",
		tag:    " v1.74.3 ",
		delete: true,
		reason: ReasonSuperseded,
	}}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			decision := Judge(test.tag, keep, releases.Deployed)
			assert.Equal(t, test.delete, decision.Delete)
			assert.Equal(t, test.reason, decision.Reason)
		})
	}
}

// TestJudgeKeepsEverythingWhenThePatchDiffers guards the most likely off-by-one: a patch
// release of the deployed minor is a different release and must be treated as one.
func TestJudgeKeepsEverythingWhenThePatchDiffers(t *testing.T) {
	releases := Releases{Deployed: "v1.76.6"}
	keep, err := releases.Keep()
	require.NoError(t, err)

	assert.False(t, Judge("v1.76.7", keep, releases.Deployed).Delete, "a newer patch is an update")
	assert.True(t, Judge("v1.76.5", keep, releases.Deployed).Delete, "an older patch is superseded")
}

// TestJudgeNeverDeletesWhatItCannotOrder: every deletion is justified by "older than what is
// deployed", so a deployed version that cannot be ordered leaves that justification
// unavailable and the answer has to be "keep".
//
// This started as a bug. Judge fell through to deleting anything not literally equal to the
// deployed value, which against an unorderable one is the entire store.
func TestJudgeNeverDeletesWhatItCannotOrder(t *testing.T) {
	keep := map[string]struct{}{"vmain": {}}

	for _, tag := range []string{"v1.76.6", "v1.75.0", "v0.1.0"} {
		decision := Judge(tag, keep, "main")
		assert.Falsef(t, decision.Delete, "tag %s was deleted against an unorderable deployed version", tag)
		assert.Equal(t, ReasonUnorderable, decision.Reason)
	}
}

// TestKeepRefusesAnUnorderableDeployedVersion is the same protection at the boundary, where
// it belongs: the precondition is established once instead of being rediscovered per tag.
func TestKeepRefusesAnUnorderableDeployedVersion(t *testing.T) {
	for _, deployed := range []string{"main", "stable", "v1.76", "latest"} {
		_, err := Releases{Deployed: deployed}.Keep()
		require.Errorf(t, err, "deployed %q was accepted as orderable", deployed)
		assert.Contains(t, err.Error(), "not a version")
	}
}
