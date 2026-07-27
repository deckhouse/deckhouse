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

package service

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/deckhouse/deckhouse/pkg/registry"
)

var (
	// ErrImageNotFound is returned when a requested tag or digest does not
	// exist. It is an alias of registry.ErrImageNotFound so callers can match
	// either.
	ErrImageNotFound = registry.ErrImageNotFound

	// ErrEmptyName is returned when a name that becomes a registry path segment
	// (module, package, plugin, extra or security image) is empty.
	ErrEmptyName = errors.New("name must not be empty")

	// ErrInvalidName is returned when such a name is not a valid OCI path
	// component.
	ErrInvalidName = errors.New("invalid registry path segment")
)

// IsNotFound reports whether err is (or wraps) ErrImageNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrImageNotFound)
}

// pathComponentRe is the OCI distribution spec grammar for one path component.
var pathComponentRe = regexp.MustCompile(`^[a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*$`)

// ValidateName checks that name can be used as a single registry path segment.
// The rule is the OCI distribution path-component grammar: lowercase
// alphanumerics, optionally separated by a period, one or two underscores, or
// one or more dashes.
func ValidateName(name string) error {
	if name == "" {
		return ErrEmptyName
	}

	if !pathComponentRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}

	return nil
}
