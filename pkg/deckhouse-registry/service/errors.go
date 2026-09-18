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

	// ErrRepositoryNotFound is returned when the repository itself is unknown,
	// as opposed to a missing tag inside one that exists. It unwraps to
	// ErrImageNotFound, so IsNotFound covers both.
	ErrRepositoryNotFound = registry.ErrRepositoryNotFound

	// ErrAccessDenied is returned when the registry refused the request for
	// lack of rights. A denied request says nothing about whether the target
	// exists, so it is never reported as "not found".
	ErrAccessDenied = registry.ErrAccessDenied

	// ErrCatalogNotSupported is returned by ListRepositories and
	// StreamRepositories against a registry with no catalog endpoint.
	ErrCatalogNotSupported = registry.ErrCatalogNotSupported

	// ErrStopStreaming, returned from a StreamTags or StreamRepositories visit
	// function, ends the walk early without being reported as a failure.
	ErrStopStreaming = registry.ErrStopStreaming

	// ErrEmptyName is returned when a name that becomes a registry path segment
	// (module, package, plugin, extra or security image) is empty.
	ErrEmptyName = errors.New("name must not be empty")

	// ErrInvalidName is returned when such a name is not a valid OCI path
	// component.
	ErrInvalidName = errors.New("invalid registry path segment")
)

// IsNotFound reports whether err is (or wraps) ErrImageNotFound. A missing
// repository satisfies it too, since ErrRepositoryNotFound unwraps to it.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrImageNotFound)
}

// IsAccessDenied reports whether the registry refused the request for lack of
// rights. Worth telling apart from IsNotFound: a wrong or expired credential
// makes every repository look empty otherwise.
func IsAccessDenied(err error) bool {
	return errors.Is(err, ErrAccessDenied)
}

// IgnoreNotFound returns nil when err reports a missing image or tag (see
// IsNotFound), and returns err unchanged otherwise. It lets a delete or cleanup
// treat something already gone as success.
func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}

	return err
}

// notFoundSentinel returns the sentinel to attach to a listing failure, or nil
// when the error is neither already classified nor recognizably a "there is
// nothing here" answer.
//
// The client this package is built on classifies its own failures, so the first
// check answers for it and nothing is re-derived — ErrRepositoryNotFound is
// covered there too, since it unwraps to ErrImageNotFound. The transport-level
// fallback exists only for a registry.Client implementation that classifies
// nothing: a test double, or a wrapper predating the sentinels.
//
// Order matters, and mirrors the client's: a refusal is not an absence. A
// token-auth registry denies out-of-scope requests with 404 as readily as 403,
// so the OCI error codes are read before the status code, and a denial is left
// unclassified here rather than reported as missing.
func notFoundSentinel(err error) error {
	if errors.Is(err, ErrImageNotFound) || errors.Is(err, ErrAccessDenied) {
		return nil
	}

	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		return nil
	}

	if transportErr.StatusCode == http.StatusUnauthorized || transportErr.StatusCode == http.StatusForbidden {
		return nil
	}

	for _, e := range transportErr.Errors {
		switch e.Code {
		case transport.UnauthorizedErrorCode, transport.DeniedErrorCode:
			return nil
		case transport.NameUnknownErrorCode:
			return ErrRepositoryNotFound
		case transport.ManifestUnknownErrorCode:
			return ErrImageNotFound
		}
	}

	if transportErr.StatusCode == http.StatusNotFound {
		return ErrImageNotFound
	}

	return nil
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
