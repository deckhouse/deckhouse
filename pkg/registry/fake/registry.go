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

package fake

import (
	"fmt"
	"strings"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// imageEntry holds a single image and its pre-computed digest.
type imageEntry struct {
	img    v1.Image
	digest v1.Hash
}

// repoStore stores the images for a single repository
// (e.g. "google-containers/pause" within the host "gcr.io").
type repoStore struct {
	mu       sync.RWMutex
	byTag    map[string]*imageEntry // tag  → image
	byDigest map[string]*imageEntry // digest-string → image
	tags     []string               // insertion-ordered tag list
}

func newRepoStore() *repoStore {
	return &repoStore{
		byTag:    make(map[string]*imageEntry),
		byDigest: make(map[string]*imageEntry),
	}
}

func (rs *repoStore) addImage(tag string, img v1.Image) error {
	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("compute digest: %w", err)
	}

	entry := &imageEntry{img: img, digest: digest}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, exists := rs.byTag[tag]; !exists {
		rs.tags = append(rs.tags, tag)
	}
	rs.byTag[tag] = entry
	rs.byDigest[digest.String()] = entry
	return nil
}

func (rs *repoStore) getByTag(tag string) (*imageEntry, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	e, ok := rs.byTag[tag]
	return e, ok
}

func (rs *repoStore) getByDigest(digest string) (*imageEntry, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	e, ok := rs.byDigest[digest]
	return e, ok
}

func (rs *repoStore) listTags() []string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	result := make([]string, len(rs.tags))
	copy(result, rs.tags)
	return result
}

func (rs *repoStore) deleteTag(tag string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if _, ok := rs.byTag[tag]; !ok {
		return false
	}
	delete(rs.byTag, tag)
	filtered := rs.tags[:0]
	for _, t := range rs.tags {
		if t != tag {
			filtered = append(filtered, t)
		}
	}
	rs.tags = filtered
	return true
}

func (rs *repoStore) deleteByDigest(digest string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if _, ok := rs.byDigest[digest]; !ok {
		return false
	}
	delete(rs.byDigest, digest)
	// Also remove any tags pointing to this digest.
	filtered := rs.tags[:0]
	for _, t := range rs.tags {
		if e, ok := rs.byTag[t]; ok && e.digest.String() == digest {
			delete(rs.byTag, t)
		} else {
			filtered = append(filtered, t)
		}
	}
	rs.tags = filtered
	return true
}

// Registry is an in-memory OCI registry scoped to a single host name.
// It is safe for concurrent use.
//
// Example – create a registry and populate it for testing:
//
//	reg := stub.NewRegistry("gcr.io")
//	img, _ := stub.NewImageBuilder().
//	    WithFile("version.json", `{"version":"v1.2.3"}`).
//	    Build()
//
//	// Add at path "google-containers/pause" with tag "3.9"
//	reg.AddImage("google-containers/pause", "3.9", img)
//
//	// Add at the registry root (empty repo path) with tag "latest"
//	reg.AddImage("", "latest", img)
type Registry struct {
	host string

	mu    sync.RWMutex
	repos map[string]*repoStore // key: canonical repository path (may be "")
}

// NewRegistry creates a new empty [Registry] for the given host.
// The host must not contain a scheme or trailing slash, e.g. "gcr.io"
// or "registry.example.com/project".
func NewRegistry(host string) *Registry {
	return &Registry{
		host:  strings.TrimSuffix(strings.TrimRight(host, "/"), "/"),
		repos: make(map[string]*repoStore),
	}
}

// Host returns the registry's host segment as provided to [NewRegistry].
func (r *Registry) Host() string {
	return r.host
}

// AddImage registers img under repoPath:tag.  repoPath may be empty (meaning
// the image lives at the root of the host).  tag must be non-empty.
// repoPath must not contain the host; it is the repository path relative to
// the host (e.g. "google-containers/pause" or "" for root-scoped images).
//
// If an image with the same tag already exists it is replaced.
func (r *Registry) AddImage(repoPath, tag string, img v1.Image) error {
	if tag == "" {
		return fmt.Errorf("stub: tag must not be empty")
	}
	repoPath = normPath(repoPath)

	r.mu.Lock()
	rs, ok := r.repos[repoPath]
	if !ok {
		rs = newRepoStore()
		r.repos[repoPath] = rs
	}
	r.mu.Unlock()

	return rs.addImage(tag, img)
}

// MustAddImage is like [AddImage] but panics on error.  Intended for test
// setup where error propagation is inconvenient.
func (r *Registry) MustAddImage(repoPath, tag string, img v1.Image) {
	if err := r.AddImage(repoPath, tag, img); err != nil {
		panic(fmt.Sprintf("stub.Registry.MustAddImage: %v", err))
	}
}

// getRepo returns the repoStore for path or nil if it does not exist.
func (r *Registry) getRepo(repoPath string) *repoStore {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.repos[normPath(repoPath)]
}

// listRepos returns all registered repository paths (relative to the host).
func (r *Registry) listRepos() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.repos))
	for p := range r.repos {
		result = append(result, p)
	}
	return result
}

// normPath strips leading and trailing slashes.
func normPath(p string) string {
	return strings.Trim(p, "/")
}
