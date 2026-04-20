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

package version

import (
	"fmt"

	semver "github.com/Masterminds/semver/v3"
)

// Compare compares two versions semantically.
// Returns -1, 0, or 1 analogous to semver.Compare - 0 if v == w, -1 if v < w, or +1 if v > w.
// If parsing of either version fails, returns 0 (fallback).
func Compare(v, w string) int {
	verV, errLeft := semver.NewVersion(v)
	verW, errRight := semver.NewVersion(w)
	if errLeft != nil || errRight != nil {
		return 0
	}

	return verV.Compare(verW)
}

func GetMax(v, w string) string {
	verV, errV := semver.NewVersion(v)
	verW, errW := semver.NewVersion(w)

	if errV != nil && errW != nil {
		return ""
	}

	if errV != nil {
		return majorMinor(verW)
	}
	if errW != nil {
		return majorMinor(verV)
	}

	if verV.LessThan(verW) {
		return majorMinor(verW)
	}
	return majorMinor(verV)
}

func GetMin(v, w string) string {
	verV, errV := semver.NewVersion(v)
	verW, errW := semver.NewVersion(w)

	if errV != nil && errW != nil {
		return ""
	}
	if errV != nil {
		return majorMinor(verW)
	}
	if errW != nil {
		return majorMinor(verV)
	}

	if verV.LessThan(verW) {
		return majorMinor(verV)
	}
	return majorMinor(verW)
}

func majorMinor(v *semver.Version) string {
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
}

func MinorInt(v string) (int, bool) {
	sv, err := semver.NewVersion(v)
	if err != nil {
		return 0, false
	}
	return int(sv.Minor()), true
}

func calculateHops(src, dst int) int {
	h := dst - src
	if h < 0 {
		h = -h
	}
	return h
}

// Hops returns the absolute number of minor version steps between sourceVersion
// and desiredVersion. Returns 0 if either version is unparseable or they are equal.
func Hops(sourceVersion, desiredVersion string) int {
	srcMinor, hasSrc := MinorInt(sourceVersion)
	dstMinor, hasDst := MinorInt(desiredVersion)
	if !hasSrc || !hasDst {
		return 0
	}
	return calculateHops(srcMinor, dstMinor)
}

// ComponentSteps returns how many minor-version upgrade steps the given component
// has already completed relative to the sourceVersion->desiredVersion migration.
// When hops == 0 (versions equal or sourceVersion unparseable), returns 1 if the
// component is at the desired version and 0 otherwise.
func ComponentSteps(componentVersion, sourceVersion, desiredVersion string) int {
	srcMinor, hasSrc := MinorInt(sourceVersion)
	dstMinor, hasDst := MinorInt(desiredVersion)
	compMinor, hasComp := MinorInt(componentVersion)
	if !hasDst || !hasComp {
		return 0
	}

	hops := 0
	if hasSrc {
		hops = calculateHops(srcMinor, dstMinor)
	}

	if hops == 0 {
		// No version migration in progress (source == desired or source is unknown).
		// Return 1 if the component is already at the desired version so that
		// StepsCompleted / totalSteps reflects 100% for a healthy, idle cluster.
		if compMinor == dstMinor {
			return 1
		}
		return 0
	}

	var steps int
	if dstMinor > srcMinor {
		steps = compMinor - srcMinor
	} else {
		steps = srcMinor - compMinor
	}
	return min(steps, hops)
}
