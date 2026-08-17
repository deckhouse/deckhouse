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

package fill

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CountHeld counts the distinct manifest digests the store holds on disk.
//
// Read from the filesystem and not through the registry API, which is the whole point. The API
// answer is not an account of what is held: a pull-through cache proxies the tag listing to its
// upstream, so asking it what it has returns what the UPSTREAM has. Measured on a cluster —
// a follower whose store held one image reported 403545, which is the number of tags in the
// development registry it proxies. That number then goes into `verifiedDigests`, and completeness
// is `held >= expectedDigests`: a store holding nothing could report itself complete and authorize
// dropping the upstream, which is exactly the failure the design forbids in as many words.
//
// Digests and not tags for a second, independent reason. The field is called `verifiedDigests` and
// is compared against `expectedDigests` — 556 distinct manifests, in the bundle this was measured
// against — while tags are a different set entirely: almost every image of the platform is
// addressed as `<base>@sha256:…` and carries no tag at all, and one digest may carry many. Counting
// tags produced 628 against an expectation of 556 on a store that was in fact complete: the right
// verdict by luck, from two numbers that do not measure the same thing.
//
// What is counted is a manifest revision the store has a link for, under `scope`, deduplicated
// across repositories: the same manifest referenced from two repositories is one digest held, and
// `expectedDigests` counts it once.
func CountHeld(root, scope string) (int32, error) {
	held, err := HeldManifests(root, scope)
	if err != nil {
		return 0, err
	}

	distinct := make(map[string]struct{}, len(held))
	for _, manifest := range held {
		distinct[manifest.Digest] = struct{}{}
	}
	return int32(len(distinct)), nil
}

// HeldManifest is one manifest a store holds, as its own filesystem records it.
type HeldManifest struct {
	// Repository the manifest is linked from. The same digest may be linked from several,
	// and each link is one thing that can be deleted.
	Repository string

	// Digest of the manifest, as `<algorithm>:<hex>`.
	Digest string
}

// HeldManifests lists the manifests the store holds on disk, under scope.
//
// The enumeration everything else here is built on, and the reason it reads files is the reason
// given on CountHeld: a registry answers "what do you have" with what it can FETCH.
//
// It is also the only way to see the manifests that matter most. The platform addresses its own
// images by digest — `<base>@sha256:…`, with no tag anywhere — so a store filled by the syncer is
// almost entirely untagged, and every question asked in terms of tags is blind to it.
func HeldManifests(root, scope string) ([]HeldManifest, error) {
	repositories := filepath.Join(root, "docker", "registry", "v2", "repositories")

	if _, err := os.Stat(repositories); errors.Is(err, os.ErrNotExist) {
		// A store that has never been written to. Empty is the honest answer, and it is not an
		// error: this is what an air-gapped replica looks like before anything reaches it.
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading the store at %s: %w", repositories, err)
	}

	var held []HeldManifest

	err := filepath.WalkDir(repositories, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != linkFile {
			return nil
		}

		repository, digest, ok := revision(repositories, path)
		if !ok {
			// Links exist for layers and tags as well; only manifest revisions are manifests.
			return nil
		}
		if !inScope(scope, repository) {
			return nil
		}

		held = append(held, HeldManifest{Repository: repository, Digest: digest})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking the store at %s: %w", repositories, err)
	}

	return held, nil
}

// HeldTag is one tag a store holds, as its own filesystem records it.
type HeldTag struct {
	// Repository the tag belongs to, as the registry names it.
	Repository string

	// Tag itself.
	Tag string
}

// HeldTags lists the tags the store holds on disk, under scope.
//
// The counterpart of CountHeld, and it exists for the same reason: asking the registry is asking
// what it can FETCH. A garbage collector that listed tags through a pull-through cache would judge
// the upstream's entire tag set — thousands of development builds — and try to delete them from a
// store that never had them, while the handful of local tags it was supposed to judge drowned among
// them. What a collector may delete has to be what this store actually holds.
func HeldTags(root, scope string) ([]HeldTag, error) {
	repositories := filepath.Join(root, "docker", "registry", "v2", "repositories")

	if _, err := os.Stat(repositories); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading the store at %s: %w", repositories, err)
	}

	var held []HeldTag

	err := filepath.WalkDir(repositories, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != linkFile {
			return nil
		}

		repository, tag, ok := currentTag(repositories, path)
		if !ok || !inScope(scope, repository) {
			return nil
		}

		held = append(held, HeldTag{Repository: repository, Tag: tag})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking the store at %s: %w", repositories, err)
	}

	return held, nil
}

// Survey is everything the loop needs to know about the store, read in ONE pass.
//
// Three separate readers used to answer these three questions — CountDeclaredHeld, CountHeld and
// MissingTags — and each walked the whole tree. On a 12 GB store that is expensive, and the loop asked
// every 30 seconds: measured on an air-gapped master, `registry-syncer` at 95% CPU with 102 minutes of
// processor time behind it on a node that had been up for 154. The work was real, the answers were the
// same three numbers, and nothing needed them recomputed that often.
//
// One walk, then, and the caller decides how often to ask. What is counted is unchanged: manifests the
// declared set names, manifests altogether, and whether the release can be resolved by tag.
type Survey struct {
	// Declared is how many manifests OF THE SET are present — the number `full` is decided from.
	Declared int32

	// Total is how many distinct manifests the store holds altogether. Reported, never decided from.
	Total int32

	// MissingTags are the asked-for tags the store cannot resolve.
	MissingTags []string
}

func Take(root, scope string, declared map[string]struct{}, tags []string) (Survey, error) {
	var survey Survey

	manifests, err := HeldManifests(root, scope)
	if err != nil {
		return survey, err
	}

	// Held means servable, not merely referenced — see blobs.complete for what that distinction cost.
	//
	// Only the declared set is checked this way. Total is a report of what the store has touched and
	// nothing decides on it, so paying for a layer walk of every manifest in the store, most of which
	// are sediment from proxying, would buy nothing.
	content := newBlobs(root)

	seen := make(map[string]struct{}, len(manifests))
	ours := make(map[string]struct{}, len(declared))
	for _, manifest := range manifests {
		seen[manifest.Digest] = struct{}{}

		if _, mine := declared[manifest.Digest]; !mine {
			continue
		}
		if _, counted := ours[manifest.Digest]; counted {
			continue
		}

		complete, err := content.complete(manifest.Digest, 0)
		if err != nil {
			return survey, err
		}
		if complete {
			ours[manifest.Digest] = struct{}{}
		}
	}
	survey.Declared = int32(len(ours))
	survey.Total = int32(len(seen))

	if len(tags) > 0 {
		missing, err := MissingTags(root, scope, tags)
		if err != nil {
			return survey, err
		}
		survey.MissingTags = missing
	}

	return survey, nil
}

// MissingTags reports which of the given tags the store does NOT hold in the scope's own repository.
//
// Read off the disk rather than asked of the registry, and that distinction is the whole point: the
// serving instance is a pull-through cache whenever there is an upstream, so asking it for a tag would
// be answered from the upstream on a miss — a store with nothing in it would confirm every tag it was
// asked about. The disk cannot do that.
//
// Only the scope's own repository, not the ones beneath it: the installer lives in a repository of its
// own, and a credential scoped to the platform is refused for it on some clusters (`401` on
// `sys/deckhouse-oss/install`, measured). Requiring a tag that cannot be fetched there would make
// completeness unreachable in exactly the way the digest count used to be.
func MissingTags(root, scope string, tags []string) ([]string, error) {
	wanted := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if tag != "" {
			wanted[tag] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}

	held, err := HeldTags(root, scope)
	if err != nil {
		return nil, err
	}

	scope = strings.Trim(scope, "/")
	for _, entry := range held {
		if strings.Trim(entry.Repository, "/") != scope {
			continue
		}
		delete(wanted, entry.Tag)
	}

	missing := make([]string, 0, len(wanted))
	for tag := range wanted {
		missing = append(missing, tag)
	}
	sort.Strings(missing)
	return missing, nil
}

// currentTag picks the repository and tag out of the path of a tag link, which distribution lays
// out as `<repositories>/<repository>/_manifests/tags/<tag>/current/link`.
//
// Only `current` counts. Alongside it distribution keeps `index/<algorithm>/<hex>/link` for every
// digest the tag has ever pointed at, and those are history: treating them as tags would make one
// tag look like as many tags as it has had values.
func currentTag(repositories, path string) (string, string, bool) {
	relative, err := filepath.Rel(repositories, path)
	if err != nil {
		return "", "", false
	}

	parts := strings.Split(filepath.ToSlash(relative), "/")
	// <repository...> / _manifests / tags / <tag> / current / link
	if len(parts) < 6 {
		return "", "", false
	}

	marker := len(parts) - 5
	if parts[marker] != "_manifests" || parts[marker+1] != "tags" || parts[marker+3] != "current" {
		return "", "", false
	}

	repository := strings.Join(parts[:marker], "/")
	if repository == "" || parts[marker+2] == "" {
		return "", "", false
	}

	return repository, parts[marker+2], true
}

// linkFile is what distribution writes to record that a digest belongs to a repository.
const linkFile = "link"

// revision picks the repository and the manifest digest out of the path of a revision link,
// which distribution lays out as
// `<repositories>/<repository>/_manifests/revisions/<algorithm>/<hex>/link`.
//
// The repository itself contains slashes, so it is found by locating the marker rather than by
// counting segments — the same reason the agent splits registry API paths by verb.
func revision(repositories, path string) (string, string, bool) {
	relative, err := filepath.Rel(repositories, path)
	if err != nil {
		return "", "", false
	}

	parts := strings.Split(filepath.ToSlash(relative), "/")
	// <repository...> / _manifests / revisions / <algorithm> / <hex> / link
	if len(parts) < 6 {
		return "", "", false
	}

	marker := len(parts) - 5
	if parts[marker] != "_manifests" || parts[marker+1] != "revisions" {
		return "", "", false
	}

	repository := strings.Join(parts[:marker], "/")
	if repository == "" {
		return "", "", false
	}

	return repository, parts[marker+2] + ":" + parts[marker+3], true
}

func roundTripper(certificateAuthority string) (http.RoundTripper, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if certificateAuthority == "" {
		return transport, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM([]byte(certificateAuthority)) {
		return nil, errors.New("the configured certificate authority is not valid PEM")
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return transport, nil
}
