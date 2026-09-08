// Copyright 2025 Flant JSC
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

package registry

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by Client implementations.
var (
	// ErrImageNotFound is returned when a requested image tag or digest does not
	// exist in the registry.
	ErrImageNotFound = errors.New("image not found")

	// ErrRepositoryNotFound is returned when the repository itself is unknown to
	// the registry (NAME_UNKNOWN), as opposed to a missing tag inside a
	// repository that does exist.
	//
	// It unwraps to ErrImageNotFound: an absent repository holds no images, so
	// callers that only branch on "the thing I asked for is not there" keep
	// working unchanged, while callers that need to tell the two apart can test
	// for this one first.
	ErrRepositoryNotFound = fmt.Errorf("repository not found (%w)", ErrImageNotFound)

	// ErrAccessDenied is returned when the registry refused the request for lack
	// of rights: HTTP 401/403, or the UNAUTHORIZED/DENIED error codes.
	//
	// Token-auth registries answer this way for anything outside the identity's
	// scope whether or not the target exists, so a denied request says nothing
	// about existence - do not treat it as "not found".
	ErrAccessDenied = errors.New("access denied")

	// ErrCatalogNotSupported is returned when a registry does not implement
	// /v2/_catalog. Docker Hub, GCR and Artifact Registry are the common cases,
	// and they answer 404 or UNSUPPORTED, which is otherwise indistinguishable
	// from a missing repository.
	ErrCatalogNotSupported = errors.New("registry does not support catalog listing")

	// ErrStopStreaming, returned from a StreamTags visit function, ends the walk
	// early without being reported as a failure. Use it when the caller has seen
	// everything it needs and further pages would be wasted round trips.
	ErrStopStreaming = errors.New("stop streaming")
)
