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
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/digests"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
)

// The names below re-export the sub-packages so that configuring a Registry and
// handling its results needs only this one import. Navigating the tree returns
// sub-package types directly; import the sub-package when you need to name one
// in your own signatures.

// BasicService is one node of the tree — a single repository. See package service.
type BasicService = service.BasicService

// DeckhouseVersion is the decoded version.json of a Deckhouse release image,
// carrying the rollout controls. See package release.
type DeckhouseVersion = release.DeckhouseVersion

// PackageVersion is the decoded version.json of a module or package release
// image, which declares only the version. See package release.
type PackageVersion = release.PackageVersion

// CanarySettings describes the canary rollout of a release on one channel.
type CanarySettings = release.CanarySettings

// Channel is a release channel name, used as a tag on release repositories.
// See package release.
type Channel = release.Channel

const (
	AlphaChannel       = release.Alpha
	BetaChannel        = release.Beta
	EarlyAccessChannel = release.EarlyAccess
	StableChannel      = release.Stable
	RockSolidChannel   = release.RockSolid
	LTSChannel         = release.LTS
)

// Channels returns every known release channel, least stable first.
func Channels() []Channel { return release.AllChannels() }

// IsChannel reports whether a tag names a release channel rather than a
// concrete version or digest.
func IsChannel(tag string) bool { return release.IsChannel(tag) }

// ModuleDefinition maps module.yaml, the legacy manifest a module release
// publishes. See package definition.
type ModuleDefinition = definition.Module

// PackageDefinition maps package.yaml, the v2 manifest a package release
// publishes for a module or an application alike. See package definition.
type PackageDefinition = definition.Package

// Digests is the decoded content of a bundle's images_digests.json, returned by
// BasicService.Digests. See package digests.
type Digests = digests.Digests

var (
	// ErrImageNotFound is returned when a tag or digest does not exist.
	ErrImageNotFound = service.ErrImageNotFound
	// ErrEmptyName is returned for an empty registry path segment.
	ErrEmptyName = service.ErrEmptyName
	// ErrInvalidName is returned for a malformed registry path segment.
	ErrInvalidName = service.ErrInvalidName
	// ErrNoVersionMetadata is returned when a release image has no version.json.
	ErrNoVersionMetadata = release.ErrNoVersionMetadata
	// ErrFileNotFound is returned when a release image does not carry a
	// requested metadata file — module.yaml, package.yaml or a changelog.
	ErrFileNotFound = release.ErrFileNotFound
	// ErrNoDigests is returned when a bundle has no images_digests.json.
	ErrNoDigests = digests.ErrNotFound
)

// IsNotFound reports whether err is (or wraps) ErrImageNotFound.
func IsNotFound(err error) bool { return service.IsNotFound(err) }

// ValidateName checks that name can be used as a single registry path segment.
func ValidateName(name string) error { return service.ValidateName(name) }
