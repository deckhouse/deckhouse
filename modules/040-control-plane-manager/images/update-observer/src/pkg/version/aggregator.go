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
	"sort"

	semver "github.com/Masterminds/semver/v3"
)

type UniqueAggregator struct {
	set         map[string]struct{}
	uniqueItems []*semver.Version
}

func NewUniqueAggregator() *UniqueAggregator {
	return &UniqueAggregator{
		set:         make(map[string]struct{}),
		uniqueItems: make([]*semver.Version, 0),
	}
}

// Set adds a version string to the aggregator.
// The item must be a valid semver string, pre-validated and normalized by version.Normalize
// (which guarantees the format "MAJOR.MINOR" and valid semver syntax).
// Calling Set with an invalid string is a programming error and will be silently ignored.
func (a *UniqueAggregator) Set(item string) {
	if _, exists := a.set[item]; exists {
		return
	}

	v, err := semver.NewVersion(item)
	if err != nil {
		// Should never happen if callers use version.Normalize before Set.
		return
	}

	a.set[item] = struct{}{}
	a.uniqueItems = append(a.uniqueItems, v)
	sort.Sort(semver.Collection(a.uniqueItems))
}

func (a *UniqueAggregator) GetMin() string {
	if len(a.uniqueItems) == 0 {
		return ""
	}

	return a.uniqueItems[0].Original()
}

func (a *UniqueAggregator) GetMax() string {
	if len(a.uniqueItems) == 0 {
		return ""
	}

	return a.uniqueItems[len(a.uniqueItems)-1].Original()
}
