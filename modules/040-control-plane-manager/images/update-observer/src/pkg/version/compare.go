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
