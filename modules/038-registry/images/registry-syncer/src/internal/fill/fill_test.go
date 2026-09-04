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
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startRegistry runs a real registry in memory, so the copy and its accounting are
// exercised against actual registry behaviour rather than a mock of it.
func startRegistry(t *testing.T) Registry {
	t.Helper()

	server := httptest.NewServer(registry.New(
		// The registry logs every request; a fill of a few hundred images would bury
		// the actual test output.
		registry.Logger(log.New(io.Discard, "", 0)),
	))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)

	return Registry{Address: parsed.Host, Insecure: true}
}

func pushImage(t *testing.T, target Registry, reference string) v1.Hash {
	t.Helper()

	image, err := random.Image(256, 1)
	require.NoError(t, err)

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Write(tag, image))

	digest, err := image.Digest()
	require.NoError(t, err)
	return digest
}

func digestOf(t *testing.T, target Registry, reference string) (v1.Hash, error) {
	t.Helper()

	tag, err := name.NewTag(target.Address+"/"+reference, name.Insecure)
	require.NoError(t, err)

	descriptor, err := remote.Get(tag)
	if err != nil {
		return v1.Hash{}, err
	}
	return descriptor.Digest, nil
}

func TestCopierFillsAnEmptyStorage(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	first := pushImage(t, source, "deckhouse/ee/registry-controller:v1")
	second := pushImage(t, source, "deckhouse/ee/release-channel:stable")

	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 2, report.Written)
	assert.EqualValues(t, 0, report.Skipped)
	assert.Empty(t, report.Failed)

	// The repository prefix is rewritten, so the cluster refers to images under a
	// fixed prefix no matter where they came from.
	got, err := digestOf(t, destination, "system/deckhouse/registry-controller:v1")
	require.NoError(t, err)
	assert.Equal(t, first, got)

	got, err = digestOf(t, destination, "system/deckhouse/release-channel:stable")
	require.NoError(t, err)
	assert.Equal(t, second, got)
}

// TestCopierIsIncrementalAndStillCountsWhatIsHeld is the accounting property the
// air-gap gate depends on: a second run copies nothing but must still report the
// storage as holding everything, because completeness is about what is THERE and
// not about what this run moved.
func TestCopierIsIncrementalAndStillCountsWhatIsHeld(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/one:v1")
	pushImage(t, source, "deckhouse/ee/two:v1")

	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}

	first, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, first.Written)

	second, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, second.Written, "the storage still holds both")
	assert.EqualValues(t, 2, second.Skipped, "and neither had to be moved again")
	assert.True(t, second.Complete())
}

// TestCopierReplacesAChangedTag covers an update arriving on an existing tag: the
// digest differs, so it has to be written even though the tag is already there.
func TestCopierReplacesAChangedTag(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/release-channel:stable")
	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	_, err := copier.Run(context.Background())
	require.NoError(t, err)

	updated := pushImage(t, source, "deckhouse/ee/release-channel:stable")

	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Written)
	assert.EqualValues(t, 0, report.Skipped)

	got, err := digestOf(t, destination, "system/deckhouse/release-channel:stable")
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

// TestCopierStaysWithinItsPrefix keeps a shared upstream from dragging everything
// on it into a cache sized for Deckhouse.
func TestCopierStaysWithinItsPrefix(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/wanted:v1")
	pushImage(t, source, "someone-else/unrelated:v1")
	// A prefix that merely starts with the same characters is a different repository.
	pushImage(t, source, "deckhouse/ee-staging/unwanted:v1")

	report, err := (&Copier{Source: source, Destination: destination, Discover: Catalogue{}}).Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Written)

	_, err = digestOf(t, destination, "system/deckhouse/wanted:v1")
	assert.NoError(t, err)

	_, err = digestOf(t, destination, "system/deckhouse/unrelated:v1")
	assert.Error(t, err)
}

func TestCopierReportsProgress(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"

	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, source, "deckhouse/ee/"+reference)
	}

	var totals []int32
	copier := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		OnProgress: func(done, _ int32) { totals = append(totals, done) },
	}

	_, err := copier.Run(context.Background())
	require.NoError(t, err)

	// A long fill has to report progress rather than going silent for however long
	// it takes.
	assert.Equal(t, []int32{1, 2, 3}, totals)
}

func TestCopierWithNothingToCopy(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	report, err := (&Copier{Source: source, Destination: destination, Discover: Catalogue{}}).Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 0, report.Written)
	assert.Empty(t, report.Failed)
	// An empty source is not a complete cache. Without this, an unreachable or
	// misconfigured upstream that lists nothing would look like a finished fill.
	assert.False(t, report.Complete())
}

func TestCopierHonoursCancellation(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "deckhouse/ee"

	for _, reference := range []string{"one:v1", "two:v1", "three:v1"} {
		pushImage(t, source, "deckhouse/ee/"+reference)
	}

	ctx, cancel := context.WithCancel(context.Background())
	copier := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		OnProgress: func(done, _ int32) {
			if done == 1 {
				cancel()
			}
		},
	}

	_, err := copier.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestReportComplete(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   bool
	}{
		{
			name:   "the whole enumerated set is in",
			report: Report{Written: 459, Total: 459},
			want:   true,
		},
		{
			name:   "more written than enumerated, which a re-run looks like",
			report: Report{Written: 460, Total: 459},
			want:   true,
		},
		{
			name:   "part of the set is missing",
			report: Report{Written: 312, Total: 459},
			want:   false,
		},
		{
			name:   "the whole set is in but something failed",
			report: Report{Written: 459, Total: 459, Failed: []string{"registry.example.com/one:v1"}},
			want:   false,
		},
		{
			name: "nothing was enumerated",
			// No set means no evidence, whatever was written or stated. Saying "complete" here would
			// authorize dropping the upstream on nothing at all.
			report: Report{Written: 459},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.report.Complete())
		})
	}
}

// store lays out a piece of distribution's on-disk format, which is what the count reads.
func store(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "data")
}

// revisionLink records that a repository holds a manifest — the one fact that means "held".
func revisionLink(t *testing.T, root, repository, digest string) {
	t.Helper()
	algorithm, hex, found := strings.Cut(digest, ":")
	require.True(t, found, "a digest is <algorithm>:<hex>")

	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_manifests", "revisions", algorithm, hex)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
}

// tagLink records that a tag points at a manifest. Deliberately available to the tests: a tag is
// exactly what must NOT be counted.
func tagLink(t *testing.T, root, repository, tag, digest string) {
	t.Helper()
	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_manifests", "tags", tag, "current")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
}

// layerLink records that a repository uses a blob. Also a link, also not a manifest.
func layerLink(t *testing.T, root, repository, digest string) {
	t.Helper()
	algorithm, hex, _ := strings.Cut(digest, ":")
	dir := filepath.Join(root, "docker", "registry", "v2", "repositories",
		filepath.FromSlash(repository), "_layers", algorithm, hex)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "link"), []byte(digest), 0o644))
}

const (
	digestOne   = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	digestTwo   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	digestThree = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
)

// TestCountHeldCountsWhatIsOnDiskAndNothingElse covers the accounting that decides whether a
// cluster may be cut off from its upstream.
func TestCountHeldCountsWhatIsOnDiskAndNothingElse(t *testing.T) {
	scope := "system/deckhouse"

	t.Run("a store that was never written to", func(t *testing.T) {
		held, err := CountHeld(store(t), scope)
		require.NoError(t, err, "an empty replica is a state, not a failure")
		assert.EqualValues(t, 0, held)
	})

	t.Run("manifests under the scope", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, "system/deckhouse", digestOne)
		revisionLink(t, root, "system/deckhouse/one", digestTwo)
		revisionLink(t, root, "system/deckhouse/group/sub", digestThree)

		held, err := CountHeld(root, scope)
		require.NoError(t, err)
		assert.EqualValues(t, 3, held, "the prefix itself counts as well as anything under it")
	})

	t.Run("the same manifest in two repositories is one digest", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, "system/deckhouse/one", digestOne)
		revisionLink(t, root, "system/deckhouse/two", digestOne)

		held, err := CountHeld(root, scope)
		require.NoError(t, err)
		assert.EqualValues(t, 1, held, "expectedDigests counts a digest once, so this must too")
	})

	t.Run("content outside the scope", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, "system/deckhouse/one", digestOne)
		revisionLink(t, root, "unrelated/image", digestTwo)
		revisionLink(t, root, "system/deckhouse-extra", digestThree)

		held, err := CountHeld(root, scope)
		require.NoError(t, err)
		assert.EqualValues(t, 1, held,
			"somebody else's content must not make this cache look complete, and a name that "+
				"merely starts like the prefix is somebody else's")
	})

	// The defect this replaced, in miniature. Tags were counted instead of manifests, and on a
	// pull-through store the tag listing is answered by the UPSTREAM — a follower holding one image
	// reported 403545, the number of tags in the registry it proxies. Completeness is `held >=
	// expectedDigests`, so that number could authorize dropping the upstream on an empty store.
	t.Run("tags are not manifests", func(t *testing.T) {
		root := store(t)
		for _, tag := range []string{"v1", "v2", "stable", "alpha", "rock-solid"} {
			tagLink(t, root, "system/deckhouse/one", tag, digestOne)
		}

		held, err := CountHeld(root, scope)
		require.NoError(t, err)
		assert.EqualValues(t, 0, held,
			"tags say what names exist, not what the store holds; five names of an absent "+
				"manifest are not five manifests")
	})

	t.Run("layers are not manifests", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, "system/deckhouse/one", digestOne)
		layerLink(t, root, "system/deckhouse/one", digestTwo)
		layerLink(t, root, "system/deckhouse/one", digestThree)

		held, err := CountHeld(root, scope)
		require.NoError(t, err)
		assert.EqualValues(t, 1, held, "an image is counted once, not once per layer")
	})

	t.Run("an empty scope counts everything", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, "system/deckhouse/one", digestOne)
		revisionLink(t, root, "unrelated/image", digestTwo)

		held, err := CountHeld(root, "")
		require.NoError(t, err)
		assert.EqualValues(t, 2, held)
	})
}

func TestRegistryOptions(t *testing.T) {
	options, err := RegistryOptions("", "", "")
	require.NoError(t, err)
	assert.Len(t, options, 1, "the transport is always configured")

	options, err = RegistryOptions("", "user", "password")
	require.NoError(t, err)
	assert.Len(t, options, 2)

	_, err = RegistryOptions("this is not a certificate", "", "")
	assert.Error(t, err, "an unusable certificate authority must fail loudly, not silently fall back")
}

func TestRewriteRepository(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		destination string
		repository  string
		want        string
	}{
		{
			name:        "prefix swapped",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee/registry-controller",
			want:        "system/deckhouse/registry-controller",
		},
		{
			name:        "the prefix itself",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee",
			want:        "system/deckhouse",
		},
		{
			name:        "no source prefix",
			destination: "system/deckhouse",
			repository:  "registry-controller",
			want:        "system/deckhouse/registry-controller",
		},
		{
			name:       "no destination prefix",
			source:     "deckhouse/ee",
			repository: "deckhouse/ee/registry-controller",
			want:       "registry-controller",
		},
		{
			name:        "a lookalike prefix is left alone",
			source:      "deckhouse/ee",
			destination: "system/deckhouse",
			repository:  "deckhouse/ee-staging/thing",
			want:        "system/deckhouse/deckhouse/ee-staging/thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copier := &Copier{
				Source:      Registry{Repository: tt.source},
				Destination: Registry{Repository: tt.destination},
			}
			assert.Equal(t, tt.want, copier.rewriteRepository(tt.repository))
		})
	}
}

// TestCompletenessDoesNotDependOnBeingToldTheNumber is the logical error the operator named.
//
// `expectedDigests` belongs to the air-gap shape of the configuration — it is how a store filled out
// of band declares what "full" means. Every cache-with-an-upstream installation omits it, and with it
// omitted nothing could ever be judged complete: no completeness means the leader never reports
// `safeToDropUpstream`, so air-gap is unreachable; and it means no follower ever replicates, because
// replication only happens from a leader that reads as full — so a leader loss costs a fill from
// scratch. Both mechanisms were switched off by an absent number.
//
// The number is knowable without being told: it is the size of the set the kept releases declare,
// which the run enumerated in order to copy it.
func TestCompletenessDoesNotDependOnBeingToldTheNumber(t *testing.T) {
	t.Run("no expectation configured, and the whole enumerated set is in", func(t *testing.T) {
		report := Report{Written: 4, Total: 4}
		assert.True(t, report.Complete(),
			"the set the run enumerated is the expectation when nobody stated one")
	})

	t.Run("no expectation configured, and part of the set is missing", func(t *testing.T) {
		report := Report{Written: 2, Total: 4}
		assert.False(t, report.Complete())
	})

	t.Run("the enumerated set is the only measure", func(t *testing.T) {
		// A stated number counts the manifests in a bundle; a run counts what the releases and the
		// modules the cluster keeps declare. A bundle holds more than any cluster needs — other
		// modules, other editions, attestations — so measuring one against the other made
		// completeness unreachable: on an air-gapped cluster, 333 declared digests held against a
		// stated 556, the store `Filling` forever while serving everything the cluster ran.
		report := Report{Written: 4, Total: 4}
		assert.True(t, report.Complete(),
			"a run that put its whole enumerated set in place is complete")
	})

	t.Run("a stated number is no substitute for a set", func(t *testing.T) {
		// It used to be the fallback, and by the owner's rule it is not one: completeness is how full
		// the store is of the images the cluster needs, and a total that counts something else must not
		// enter the decision — not even when there is nothing else to go on.
		written := Report{Written: 556}
		assert.False(t, written.Complete(),
			"writing as many images as an operator counted in a bundle says nothing about the set")
	})

	t.Run("nothing enumerated and nothing configured is not completeness", func(t *testing.T) {
		report := Report{}
		assert.False(t, report.Complete(),
			"an empty set is not a full cache, and saying it is would authorize dropping the upstream")
	})

	t.Run("a failure is never complete", func(t *testing.T) {
		report := Report{Written: 4, Total: 4, Failed: []string{"repo@sha256:one"}}
		assert.False(t, report.Complete())
	})
}

// TestCopyingFromASourceStillFillingReportsWhatIsMissingAsPending is what makes ahead-of-time
// replication possible at all.
//
// A follower enumerates the same set its leader is filling towards, so while the leader is part-way
// through, part of that set is simply not there yet. Counted as failure — which is what it was — every
// early replication looked broken, and followers were therefore kept from starting until the leader
// was complete: idle for as long as a fill takes, and holding nothing if the leader died meanwhile.
// Measured on a cluster as three replicas with 428, 337 and 333 digests, none full, none moving.
func TestCopyingFromASourceStillFillingReportsWhatIsMissingAsPending(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)
	source.Repository = "system/deckhouse"
	destination.Repository = "system/deckhouse"

	// The leader holds one of the two declared images so far.
	present := pushByDigest(t, source, "system/deckhouse")
	missing := "sha256:1111111111111111111111111111111111111111111111111111111111111111"

	copier := &Copier{
		Source: source, Destination: destination,
		Discover: explicit{
			source.Address + "/system/deckhouse@" + present.String(),
			source.Address + "/system/deckhouse@" + missing,
		},
	}

	report, err := copier.Run(context.Background())
	require.NoError(t, err)

	assert.EqualValues(t, 1, report.Written, "what the source has is copied")
	assert.Len(t, report.Pending, 1, "what it does not have yet is pending")
	assert.Empty(t, report.Failed, "and is not a failure of the copy")
	assert.False(t, report.Complete(), "a partial copy is still not a complete cache")
}

// explicit is a discoverer that returns exactly the references given, so a test can describe a source
// mid-fill without depending on how a release is enumerated.
type explicit []string

func (e explicit) Discover(context.Context, Registry, *remote.Puller) ([]name.Reference, error) {
	references := make([]name.Reference, 0, len(e))
	for _, raw := range e {
		reference, err := name.ParseReference(raw, name.Insecure)
		if err != nil {
			return nil, err
		}
		references = append(references, reference)
	}
	return references, nil
}

// blob writes a blob's content, which is what makes a digest servable rather than merely referenced.
func blob(t *testing.T, root, digest string, content []byte) {
	t.Helper()
	algorithm, hex, found := strings.Cut(digest, ":")
	require.True(t, found, "a digest is <algorithm>:<hex>")

	dir := filepath.Join(root, "docker", "registry", "v2", "blobs", algorithm, hex[:2], hex)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data"), content, 0o644))
}

// imageManifest writes a manifest naming a config and one layer, and returns it as JSON.
func imageManifest(config, layer string) []byte {
	return []byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json",` +
		`"config":{"digest":"` + config + `"},"layers":[{"digest":"` + layer + `"}]}`)
}

// TestTakeCountsOnlyImagesTheStoreCanServe is the accounting that authorizes cutting a cluster off
// from its upstream, against the one thing that makes it wrong.
//
// A pull-through cache writes the revision link as soon as it has served a manifest and fetches the
// layers only when somebody asks. Counting links therefore counts manifests the store cannot serve.
// Measured on `ly-mmc`: three replicas reporting `full`, `safeToDropUpstream: true` and 400 verified
// digests each, holding 333 MB — 332 manifests with 61 layer links between them. With the upstream
// gone no node could pull anything, by tag or by digest, and the replicas answered each other's blob
// requests with 404 because none of them had the data.
func TestTakeCountsOnlyImagesTheStoreCanServe(t *testing.T) {
	const (
		scope   = "system/deckhouse"
		config  = "sha256:aaaa111111111111111111111111111111111111111111111111111111111111"
		layer   = "sha256:bbbb222222222222222222222222222222222222222222222222222222222222"
		missing = "sha256:cccc333333333333333333333333333333333333333333333333333333333333"
	)

	t.Run("a manifest whose layers are on disk", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, scope, digestOne)
		blob(t, root, digestOne, imageManifest(config, layer))
		blob(t, root, config, []byte(`{}`))
		blob(t, root, layer, []byte("layer"))

		survey, err := Take(root, scope, map[string]struct{}{digestOne: {}}, nil)
		require.NoError(t, err)
		assert.EqualValues(t, 1, survey.Declared, "everything it names is here, so it is held")
		assert.EqualValues(t, 1, survey.Total)
	})

	t.Run("a manifest whose layer was never fetched", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, scope, digestOne)
		blob(t, root, digestOne, imageManifest(config, missing))
		blob(t, root, config, []byte(`{}`))

		survey, err := Take(root, scope, map[string]struct{}{digestOne: {}}, nil)
		require.NoError(t, err)
		assert.EqualValues(t, 0, survey.Declared,
			"a manifest without its layers cannot be served, and counting it authorizes an air-gap the cluster cannot survive")
		assert.EqualValues(t, 1, survey.Total, "the store did touch it, and Total reports what it touched")
	})

	t.Run("a manifest the store never fetched at all", func(t *testing.T) {
		root := store(t)
		revisionLink(t, root, scope, digestOne)

		survey, err := Take(root, scope, map[string]struct{}{digestOne: {}}, nil)
		require.NoError(t, err)
		assert.EqualValues(t, 0, survey.Declared, "a link without the manifest itself holds nothing")
	})

	t.Run("an index counts only when every image under it is complete", func(t *testing.T) {
		root := store(t)
		index := []byte(`{"mediaType":"application/vnd.oci.image.index.v1+json",` +
			`"manifests":[{"digest":"` + digestTwo + `"},{"digest":"` + digestThree + `"}]}`)

		revisionLink(t, root, scope, digestOne)
		blob(t, root, digestOne, index)

		// One child complete, the other missing its layer.
		blob(t, root, digestTwo, imageManifest(config, layer))
		blob(t, root, digestThree, imageManifest(config, missing))
		blob(t, root, config, []byte(`{}`))
		blob(t, root, layer, []byte("layer"))

		survey, err := Take(root, scope, map[string]struct{}{digestOne: {}}, nil)
		require.NoError(t, err)
		assert.EqualValues(t, 0, survey.Declared, "half an index is not an image anybody can pull")

		// And with the second child's layer in place it becomes servable.
		blob(t, root, missing, []byte("layer"))
		survey, err = Take(root, scope, map[string]struct{}{digestOne: {}}, nil)
		require.NoError(t, err)
		assert.EqualValues(t, 1, survey.Declared)
	})
}

// TestACopyRepairsAManifestWithoutItsLayers is what lets a store filled by proxying ever become
// complete.
//
// The destination is asked whether it already holds a digest, and a registry answers yes for a
// manifest it has without a single layer — exactly what a pull-through cache leaves behind. Skipping
// on that answer means the fill reports everything as already present and copies nothing, so the
// store stays unservable however often it runs. Measured on `ly-mmc` the moment completeness started
// reading layers: the leader correctly reported holding none of the set, began a fill, and wrote
// nothing at all — 333 MB before, 333 MB after, zero writes to the registry.
func TestACopyRepairsAManifestWithoutItsLayers(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/registry-controller:v1")

	// Copied once with no StoreDir: the destination really holds it afterwards.
	plain := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	first, err := plain.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, first.Written)

	second, err := plain.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, second.Skipped, "without a store to read, present still means done")

	// And now the same copy against a store directory that holds no blobs at all — which is what a
	// proxied store looks like on disk. The image must be copied again rather than skipped.
	repairing := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		StoreDir: t.TempDir(),
	}
	third, err := repairing.Run(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 0, third.Skipped,
		"a manifest whose layers are not on disk must be re-copied, or the store can never repair itself")
	assert.EqualValues(t, 1, third.Written)
}

// TestACopyBringsTheLayersWithIt is the property every other check here assumed and none of them
// tested: that a copied image can be pulled from the destination afterwards.
//
// `remote.Pusher.Push` uploads what its argument depends on, and a `*remote.Descriptor` — which is
// what a fetch returns — depends on nothing at all. Pushing it puts the manifest in the destination
// and leaves the layers where they were. The store then holds a complete-looking set of manifests
// that only resolve while the upstream is still reachable, which is the one condition an air-gapped
// cluster does not have.
//
// Measured on `ly-mmc`: a fill reporting `written=400, skipped=0` — the entire set, deliberately
// re-copied — that left the store at the same 333 MB and the same 450 blobs it had before.
func TestACopyBringsTheLayersWithIt(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/registry-controller:v1")

	copier := &Copier{Source: source, Destination: destination, Discover: Catalogue{}}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, report.Written)

	// Asked of the destination over HTTP, blob by blob, rather than through a client that could
	// answer from anywhere else: what is being tested is whether the bytes are THERE.
	reference, err := name.NewTag(
		source.Address+"/deckhouse/ee/registry-controller:v1", name.Insecure)
	require.NoError(t, err)
	original, err := remote.Image(reference)
	require.NoError(t, err)

	layers, err := original.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers, "the fixture itself has to have layers, or this proves nothing")

	for _, layer := range layers {
		digest, err := layer.Digest()
		require.NoError(t, err)

		response, err := http.Get(fmt.Sprintf(
			"http://%s/v2/system/deckhouse/registry-controller/blobs/%s", destination.Address, digest))
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		assert.Equal(t, http.StatusOK, response.StatusCode,
			"the destination does not hold layer %s, so only the manifest was copied", digest)
	}
}

// manifestOnly is a manifest with nothing behind it, which is what a registry ends up holding when a
// pull-through cache serves one metadata request, or when a copy is interrupted after the manifest
// went in. `remote.Put` writes such a value verbatim: a Taggable that is neither an image nor an
// index has no dependencies for the pusher to walk.
type manifestOnly struct {
	raw       []byte
	mediaType types.MediaType
}

func (m manifestOnly) RawManifest() ([]byte, error)        { return m.raw, nil }
func (m manifestOnly) MediaType() (types.MediaType, error) { return m.mediaType, nil }

// TestACopyRepairsAnImageWhoseManifestArrivedAlone is the case the repair path above could not
// actually repair.
//
// Deciding to re-copy is not the same as re-copying: `remote.Pusher.Push` asks the destination
// whether it holds the manifest and returns success as soon as it does, without writing the blobs the
// manifest names. So the fill reported writing an image every pass while the destination stayed
// unpullable — measured on `ly-mmc` as two followers stuck at 398 and 397 of 403 for forty minutes,
// each missing exactly one blob: the image config, with every layer already there.
//
// The assertion is deliberately about pulling and not about counters. The counter was right the whole
// time this defect existed; the store was empty of what the counter claimed.
func TestACopyRepairsAnImageWhoseManifestArrivedAlone(t *testing.T) {
	source := startRegistry(t)
	destination := startRegistry(t)

	source.Repository = "deckhouse/ee"
	destination.Repository = "system/deckhouse"

	pushImage(t, source, "deckhouse/ee/registry-controller:v1")

	sourceTag, err := name.NewTag(source.Address+"/deckhouse/ee/registry-controller:v1", name.Insecure)
	require.NoError(t, err)
	descriptor, err := remote.Get(sourceTag)
	require.NoError(t, err)

	destinationTag, err := name.NewTag(
		destination.Address+"/system/deckhouse/registry-controller:v1", name.Insecure)
	require.NoError(t, err)
	require.NoError(t, remote.Put(destinationTag,
		manifestOnly{raw: descriptor.Manifest, mediaType: descriptor.MediaType}))

	// The manifest is there and the store holds none of its blobs — exactly the state that used to
	// repeat forever.
	copier := &Copier{
		Source: source, Destination: destination, Discover: Catalogue{},
		StoreDir: t.TempDir(),
	}
	report, err := copier.Run(context.Background())
	require.NoError(t, err)
	require.Empty(t, report.Failed)
	assert.EqualValues(t, 1, report.Written)

	image, err := remote.Image(destinationTag)
	require.NoError(t, err)

	_, err = image.RawConfigFile()
	require.NoError(t, err, "the config blob has to travel too, and it is not one of the layers")

	layers, err := image.Layers()
	require.NoError(t, err)
	require.NotEmpty(t, layers)
	for _, layer := range layers {
		digest, err := layer.Digest()
		require.NoError(t, err)

		content, err := layer.Compressed()
		require.NoError(t, err, "layer %s is named by the manifest but absent from the destination", digest)
		require.NoError(t, content.Close())
	}
}
