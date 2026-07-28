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
	"net/http"
	"regexp"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

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

// isNotFound recognizes a registry answer meaning "there is nothing here":
// either the sentinel already, or the transport-level codes a registry uses for
// a missing repository or manifest. Operations that the underlying client does
// not already map — listing, in particular — run their error through this so
// callers only ever match ErrImageNotFound.
func isNotFound(err error) bool {
	if errors.Is(err, ErrImageNotFound) {
		return true
	}

	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		return false
	}

	if transportErr.StatusCode == http.StatusNotFound {
		return true
	}

	for _, e := range transportErr.Errors {
		switch e.Code {
		case transport.NameUnknownErrorCode, transport.ManifestUnknownErrorCode:
			return true
		}
	}

	return false
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
