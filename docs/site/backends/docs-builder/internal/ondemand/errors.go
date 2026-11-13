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

package ondemand

import (
	"errors"

	"github.com/flant/docs-builder/internal/docs"
)

// The failures of the ports of the store, in its own terms: what the cluster,
// the registry or S3 answered is wrapped in one of these by the code that
// reads them, so the store acts on a failure without knowing where it came
// from. kindOf tells what each one means.
var (
	// ErrNoVersions means the module has no versions to render: no source of
	// it, or no release repository in its sources (TagLister, the lookups of
	// a TargetPolicy).
	ErrNoVersions = errors.New("module has no published versions")
	// ErrVersionNotFound means no source of the module has an image of the
	// version (RenderFunc).
	ErrVersionNotFound = errors.New("module version not found")
	// ErrFetch marks a failure to get the documentation from the registry,
	// as opposed to a failure to build the pages (RenderFunc).
	ErrFetch = errors.New("fetch documentation")
	// ErrPartialList means some sources of the module did not answer: the
	// tags come from the others (TagLister).
	ErrPartialList = errors.New("some sources of the module did not answer")
	// ErrNoChannel means no source of the module has a release channel tag
	// (the lookups of a TargetPolicy).
	ErrNoChannel = errors.New("module has no release channel tags")
	// ErrChannelsGone means no source of the module has a release channel
	// tag, though the last lookup found some: a release repository uploaded
	// anew answers "not found" for a while, and the newest version a module
	// without channels gets may be a pre-release. Like a registry failure, it
	// keeps the last target and channels of the module for the pass; gone on
	// the next lookup too, the tags are ErrNoChannel.
	ErrChannelsGone = errors.New("release channel tags found last time are gone")
	// ErrColdNotFound means the cold cache has no object with the key
	// (ColdCache).
	ErrColdNotFound = errors.New("no such object in the cold cache")
	// ErrColdNotReady means the cold cache is not set up yet: it answers so
	// to every call until it is (ColdCache).
	ErrColdNotReady = errors.New("the cold cache is not ready")
)

// kind is what a failure means to the store.
type kind int

const (
	// passing: the registry, S3 or the pod failed, and the next try may go
	// well.
	passing kind = iota
	// missing: the module, the version or the object is not there.
	missing
	// noDocs: the version has no documentation pages.
	noDocs
	// unrenderable: the template can not render the version, or not with
	// the AI export, until the template or the image changes.
	unrenderable
	// notReady: the cold cache is not set up yet.
	notReady
)

// kindOf classifies a failure by the errors above and those of docs and of
// the store it wraps. A new kind of error goes here, and every decision on
// failures follows: lasting, Unpublished, coldMissing, the status of a
// render and its reason.
func kindOf(err error) kind {
	switch {
	case errors.Is(err, ErrNoVersions), errors.Is(err, ErrVersionNotFound), errors.Is(err, ErrColdNotFound):
		return missing
	case errors.Is(err, docs.ErrNoPages):
		return noDocs
	case errors.Is(err, docs.ErrBuild), errors.Is(err, docs.ErrDocsTooLarge), errors.Is(err, errNoAIExport):
		return unrenderable
	case errors.Is(err, ErrColdNotReady):
		return notReady
	}

	return passing
}

// lasting reports whether a render failed for a reason that does not change
// until the template or the image does: trying it again at once fails again.
func lasting(err error) bool {
	k := kindOf(err)

	return k == missing || k == noDocs || k == unrenderable
}

// Unpublished tells an error of a listing that means the module has no
// versions to render.
func Unpublished(err error) bool {
	return kindOf(err) == missing
}

// coldMissing reports whether err means the object is not in the cold cache,
// or the cache is not set up yet.
func coldMissing(err error) bool {
	k := kindOf(err)

	return k == missing || k == notReady
}
