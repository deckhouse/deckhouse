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

// Package gc reclaims the disk a long-lived cache accumulates.
//
// Every release adds a slice of the repository and nothing has ever removed one, so a
// cluster that lives for years fills its store and then stops being able to pull. What is
// reclaimable is the slices belonging to releases the cluster has moved past.
//
// The whole package is built around one asymmetry. Deleting a blob the cluster still needs
// is, in an air-gapped cluster, unrecoverable without another `d8 mirror push` — while
// keeping a blob nobody needs costs disk and nothing else. So every judgement here is made
// in one direction: anything not positively identified as belonging to a superseded release
// is kept, and a run that cannot establish what to keep does nothing at all.
package gc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Releases is what the cluster says about itself: the version it is running, and the one it
// came from.
type Releases struct {
	// Deployed is the version currently in use. Empty means unknown.
	Deployed string

	// Previous is the most recent superseded version, kept so that a rollback does not
	// have to re-download the release it rolls back to. Empty when the cluster has never
	// updated.
	Previous string
}

// Keep is the set of versions a run must not touch.
//
// Returns an error rather than an empty set when the deployed version is unknown, because
// those two are opposite instructions: an empty keep-set with a store full of versions
// would delete everything.
func (r Releases) Keep() (map[string]struct{}, error) {
	if strings.TrimSpace(r.Deployed) == "" {
		return nil, fmt.Errorf("the deployed version is unknown, so there is nothing to keep against")
	}
	// Checked here, once, rather than discovered per tag: every deletion is justified by
	// "older than what is deployed", and a deployed version that is not a version cannot
	// order anything. Without this the keep-set would contain one unorderable entry and
	// every real version in the store would fall through to being deleted.
	if !version.MatchString(normalize(r.Deployed)) {
		return nil, fmt.Errorf(
			"the deployed version %q is not a version, so no tag can be judged older than it", r.Deployed)
	}

	keep := map[string]struct{}{normalize(r.Deployed): {}}
	if previous := normalize(r.Previous); previous != "" {
		keep[previous] = struct{}{}
	}
	return keep, nil
}

// version matches the tags this package is willing to reason about: a semantic version,
// with or without a leading `v`, and nothing else in the tag.
//
// Deliberately narrow. A tag that does not match is not garbage — it is a tag whose meaning
// this package does not know, which is a different thing and is kept. Release channel names
// (`stable`, `alpha`), floating tags, and anything an operator pushed by hand all fall here.
var version = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z.\-]+)?$`)

// normalize reduces a version to a comparable form, so that `v1.2.3` and `1.2.3` are the
// same version — which they are, and the registry holds both spellings depending on who
// pushed.
func normalize(tag string) string {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return ""
	}
	return "v" + strings.TrimPrefix(trimmed, "v")
}

// Decision is what a run should do with one tag.
type Decision struct {
	// Delete is whether the tag may go.
	Delete bool

	// Reason says why, for the log. A kept tag has one too: "why was this not reclaimed"
	// is the question an operator asks when the disk is still full.
	Reason string
}

// Reasons a tag is kept or deleted.
const (
	ReasonCurrentRelease  = "belongs to the deployed release"
	ReasonPreviousRelease = "belongs to the previous release, kept for a rollback"
	ReasonNotAVersion     = "not a version tag, so what it means is unknown"
	ReasonNewerRelease    = "newer than the deployed release, so it is an update in progress"
	ReasonSuperseded      = "belongs to a release the cluster has moved past"
	ReasonUnorderable     = "cannot be compared with the deployed release, so it is not known to be old"
)

// Judge decides one tag against the keep-set.
func Judge(tag string, keep map[string]struct{}, deployed string) Decision {
	if !version.MatchString(strings.TrimSpace(tag)) {
		// The conservative half of the whole package. A tag whose meaning is unknown is
		// kept, because the cost of being wrong is asymmetric: an unnecessary gigabyte
		// against a cluster that cannot pull.
		return Decision{Reason: ReasonNotAVersion}
	}

	normalized := normalize(tag)
	if _, ok := keep[normalized]; ok {
		if normalized == normalize(deployed) {
			return Decision{Reason: ReasonCurrentRelease}
		}
		return Decision{Reason: ReasonPreviousRelease}
	}

	// A version ahead of what is deployed is an update the cluster is in the middle of, or
	// about to start. Deleting it would make the update re-download everything it had
	// already fetched, and in air-gap it would delete a release that was pushed on purpose
	// and cannot be fetched again.
	newer, err := isNewer(normalized, normalize(deployed))
	switch {
	case err != nil:
		// The comparison could not be made, so the one justification for deleting this tag
		// is unavailable. Callers are stopped from reaching here by Keep, and this is the
		// second line: an unorderable deployed version must not turn into "delete
		// everything that is not literally it".
		return Decision{Reason: ReasonUnorderable}
	case newer:
		return Decision{Reason: ReasonNewerRelease}
	}

	return Decision{Delete: true, Reason: ReasonSuperseded}
}

// isNewer compares two normalized versions.
//
// Only the numeric parts are compared, and a pre-release suffix is ignored rather than
// ordered: getting pre-release precedence subtly wrong here would delete something, and the
// question being asked only needs "is this ahead of what we run".
func isNewer(candidate, reference string) (bool, error) {
	left, err := parse(candidate)
	if err != nil {
		return false, err
	}
	right, err := parse(reference)
	if err != nil {
		return false, err
	}

	for i := range left {
		switch {
		case left[i] > right[i]:
			return true, nil
		case left[i] < right[i]:
			return false, nil
		}
	}
	return false, nil
}

func parse(tag string) ([3]int, error) {
	var parsed [3]int

	match := version.FindStringSubmatch(tag)
	if match == nil {
		return parsed, fmt.Errorf("%q is not a version", tag)
	}
	for i := 0; i < 3; i++ {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return parsed, fmt.Errorf("%q is not a version: %w", tag, err)
		}
		parsed[i] = value
	}
	return parsed, nil
}
