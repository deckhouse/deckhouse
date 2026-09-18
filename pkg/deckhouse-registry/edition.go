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

package dhregistry

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrUnknownEdition is returned by ParseEdition for an unrecognized edition.
var ErrUnknownEdition = errors.New("unknown deckhouse edition")

// Edition is the Deckhouse distribution edition. It forms the sub-path under
// the registry root for everything that is edition-scoped:
//
//	registry.deckhouse.io/deckhouse/<edition>/...
type Edition string

const (
	// NoEdition means the registry root is not edition-scoped. It is used for
	// custom/dev roots such as dev-registry.deckhouse.io/sys/deckhouse-oss,
	// where all edition-scoped services hang directly off the root.
	NoEdition Edition = ""

	CEEdition     Edition = "ce"
	BEEdition     Edition = "be"
	SEEdition     Edition = "se"
	SEPlusEdition Edition = "se-plus"
	EEEdition     Edition = "ee"
	FEEdition     Edition = "fe"
)

// knownEditions lists every valid edition, in ascending order of coverage.
var knownEditions = []Edition{CEEdition, BEEdition, SEEdition, SEPlusEdition, EEEdition, FEEdition}

// Editions returns all valid editions. NoEdition is not included.
func Editions() []Edition {
	return slices.Clone(knownEditions)
}

// String returns the registry path segment for the edition ("" for NoEdition).
func (e Edition) String() string {
	return string(e)
}

// IsValid reports whether e is one of the known editions. NoEdition is not valid.
func (e Edition) IsValid() bool {
	return slices.Contains(knownEditions, e)
}

// ParseEdition converts a string to an Edition, case-insensitively. An empty
// string yields NoEdition without an error; anything unrecognized is an error.
func ParseEdition(s string) (Edition, error) {
	if s == "" {
		return NoEdition, nil
	}

	candidate := Edition(strings.ToLower(strings.TrimSpace(s)))
	if !candidate.IsValid() {
		return NoEdition, fmt.Errorf("%w: %q", ErrUnknownEdition, s)
	}

	return candidate, nil
}

// SplitEdition splits a registry repository path into its non-edition root and
// the edition it is scoped to. When the last segment is not a known edition the
// path is returned unchanged along with NoEdition.
//
//	registry.deckhouse.io/deckhouse/ee/ -> "registry.deckhouse.io/deckhouse", EEEdition
//	dev-registry.deckhouse.io/sys/deckhouse-oss -> unchanged, NoEdition
func SplitEdition(repo string) (string, Edition) {
	trimmed := strings.TrimSuffix(repo, "/")

	idx := strings.LastIndex(trimmed, "/")
	if idx == -1 {
		return trimmed, NoEdition
	}

	candidate := Edition(trimmed[idx+1:])
	if !candidate.IsValid() {
		return trimmed, NoEdition
	}

	return trimmed[:idx], candidate
}
