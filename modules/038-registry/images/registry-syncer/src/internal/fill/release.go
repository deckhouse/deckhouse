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
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Paths inside an installer image that name the platform's image set.
//
// The same two files `d8 mirror pull` reads, in the same order of preference. They are how a
// release says what it consists of, which is the only account of that a registry can be asked
// for without being allowed to list it.
const (
	imagesTagsFile    = "deckhouse/candi/images_tags.json"
	imagesDigestsFile = "deckhouse/candi/images_digests.json"

	// platformDigestsFile is the same account of the image set, inside the platform's own image.
	//
	// The same file: measured on a cluster, `/deckhouse/modules/images_digests.json` in the running
	// deckhouse image is byte for byte the 38277-byte `deckhouse/candi/images_digests.json` of the
	// installer of that version. It is also the authority rather than a copy of one — the
	// controller decides which images to load by reading exactly this path
	// (deckhouse-controller/internal/packages/loader, `embeddedDir = "modules"`).
	//
	// Preferred over the installer because of what it costs to read. The installers live in a
	// repository of their own, `<path>/install`, and a credential scoped to the platform's
	// repository is refused for it: measured on a cluster, the fill of the running version failed
	// with `401 Unauthorized` on `sys/deckhouse-oss/install`. The platform image needs no
	// permission the cluster does not already have — it is the image the cluster is running, pulled
	// through this very store.
	platformDigestsFile = "deckhouse/modules/images_digests.json"
)

// installerRepository is where an installer image for a version lives, under the platform's
// own repository.
const installerRepository = "install"

// Release enumerates what a set of releases consists of, by reading it out of their
// installers.
//
// This is how `d8 mirror pull` decides what to mirror, and the reason it does it this way is
// the reason this does: a release declares its own images, so nothing has to be inferred from
// what a registry happens to hold, and nothing has to be permitted beyond pulling the images
// the cluster already pulls.
//
// It also makes completeness meaningful. "Everything in the upstream" is not a set a cache can
// be complete with respect to — it changes without the cluster, and it includes versions the
// cluster will never run. "Everything these releases declare" is a fixed, countable set, and
// is exactly what has to be present before an air-gapped cluster can be cut off from the
// upstream.
//
// Deliberately the deployed release and the one before it, the same pair the collector keeps:
// the fill puts in what the collector would not throw away, so the two cannot disagree about
// what the store is for. Keeping the previous one is what makes a rollback possible without a
// download.
type Release struct {
	// Versions are the release versions to enumerate, as they appear in the registry's
	// tags. Empty is an error rather than "everything": a fill that silently copied
	// nothing would report a complete cache on an empty store.
	Versions []string

	// Modules are the modules the cluster keeps, which a release does not account for.
	//
	// They belong in the same enumeration rather than beside it, because every consumer of this set
	// has to see the same one: the copy that fills a store, the count that judges it complete, and the
	// collection that decides what may be deleted. Two of them disagreeing about what the set is was
	// how a cluster came to move the lease between three replicas every twenty seconds — and the
	// version of that bug which matters most here would be silent: a store judged complete on the
	// platform's images alone, cut off from its upstream, and missing every image its modules run.
	Modules []ModuleRef
}

func (r Release) Discover(
	ctx context.Context, source Registry, puller *remote.Puller,
) ([]name.Reference, error) {
	if len(r.Versions) == 0 {
		return nil, errors.New("no release version to enumerate, so there is nothing to copy")
	}

	registry, err := parseRegistry(source)
	if err != nil {
		return nil, err
	}
	// Both built from the registry, because `Repository` embeds it: calling `Repo` on a
	// repository resolves against the registry root and silently drops the prefix, which is
	// how the installer was first looked for at `<registry>/install`.
	platform := registry.Repo(source.Repository)
	installers := registry.Repo(source.Repository, installerRepository)

	// Deduplicated across versions, because consecutive releases share most of their
	// images: two versions of Deckhouse differ in a handful of digests and agree on the
	// rest, and copying the intersection twice would double the work and the reported total.
	seen := make(map[string]struct{})
	var references []name.Reference
	add := func(reference name.Reference) {
		if _, already := seen[reference.String()]; already {
			return
		}
		seen[reference.String()] = struct{}{}
		references = append(references, reference)
	}

	for _, version := range r.Versions {
		if version == "" {
			continue
		}

		// The release's own image, by tag.
		add(platform.Tag(version))

		declared, from, err := r.declaredImages(ctx, platform, installers, puller, version)
		if err != nil {
			return nil, fmt.Errorf("reading the image set of %s: %w", version, err)
		}

		// The installer is READ but not kept: it is used by the installation process and by nothing in a
		// running cluster, so a cluster works without it and its absence costs nothing.
		//
		// Keeping it used to be conditional, which is worse than either answer: the installers live in
		// a repository of their own and a credential scoped to the platform's is refused for it
		// (`401 Unauthorized` on `sys/deckhouse-oss/install`, measured), so on some clusters the set
		// contained a reference that could never be copied — the fill never completed, the upstream was
		// never dropped, and no follower ever replicated. On others it silently did contain it, which
		// meant two clusters of the same version disagreed about what "the whole set" was.
		//
		// `from` still names where the set was read, and that stays: reading the installer is a pull
		// from the upstream, not something the store has to hold.
		_ = from

		for _, reference := range declared {
			add(reference)
		}
	}

	// The modules the cluster keeps, in the same set and deduplicated against it.
	moduleReferences, err := ModuleReferences(ctx, source, puller, r.Modules)
	if err != nil {
		return nil, err
	}
	for _, reference := range moduleReferences {
		add(reference)
	}

	return references, nil
}

// declaredImages reads the image set a version declares out of its installer.
// declaredImages enumerates one version's image set, and says which image it was read out of.
func (r Release) declaredImages(
	ctx context.Context, platform, installers name.Repository, puller *remote.Puller, version string,
) ([]name.Reference, string, error) {
	// The platform's own image first, the installer only if that fails. Both carry the same
	// account of the set; the difference is that the first is the image this cluster runs, so it is
	// present by definition and reachable with the credentials already in use, while the second
	// lives in a repository those credentials may not cover.
	source := platform.Tag(version)
	tags, digests, err := r.imageSet(ctx, puller, source, platformDigestsFile)
	if err != nil {
		installer := installers.Tag(version)

		fallbackTags, fallbackDigests, fallbackErr := r.imageSet(
			ctx, puller, installer, imagesDigestsFile)
		if fallbackErr != nil {
			// Both named, because the two failures mean different things and the operator needs
			// to see which one to act on: the platform image is expected to be there, and the
			// installer is expected to be permitted.
			return nil, "", fmt.Errorf(
				"the image set could not be read from %s (%w) nor from %s (%w)",
				source, err, installer, fallbackErr)
		}
		source, tags, digests = installer, fallbackTags, fallbackDigests
	}

	// Tags take precedence over digests, as in `d8 mirror pull`: where a release records
	// both, the tags are the ones it is published under.
	var references []name.Reference
	switch {
	case len(tags) > 0:
		for _, tag := range tags {
			references = append(references, platform.Tag(tag))
		}
	case len(digests) > 0:
		for _, digest := range digests {
			parsed, err := v1.NewHash(digest)
			if err != nil {
				return nil, "", fmt.Errorf("%q is not a digest: %w", digest, err)
			}
			references = append(references, platform.Digest(parsed.String()))
		}
	default:
		// Refused rather than treated as an empty set. An image this cannot be read out of
		// means the assumption this whole enumeration rests on has stopped holding, and
		// reporting "nothing to copy" would make an empty cache look complete.
		return nil, "", fmt.Errorf("%s declares no image set", source)
	}

	return references, source.String(), nil
}

// imageSet reads the account of the image set out of one image.
//
// The digests path differs between the two images that carry it, so it is a parameter; the tags
// file, which `d8 mirror pull` prefers where a release publishes one, keeps its own path in both.
func (r Release) imageSet(
	ctx context.Context, puller *remote.Puller, reference name.Reference, digestsPath string,
) ([]string, []string, error) {
	descriptor, err := puller.Get(ctx, reference)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", reference, err)
	}
	image, err := descriptor.Image()
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", reference, err)
	}

	tags, digests, err := readImageSet(mutate.Extract(image), digestsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the image set out of %s: %w", reference, err)
	}
	if len(tags) == 0 && len(digests) == 0 {
		// An empty answer is not an answer here: it would enumerate nothing and make an empty
		// cache look complete. Reported as a failure so the other source is tried.
		return nil, nil, fmt.Errorf("%s carries neither %s nor %s", reference, imagesTagsFile, digestsPath)
	}
	return tags, digests, nil
}

// readImageSet pulls the two metadata files out of a flattened image.
//
// Returns the values only: the files are keyed by module and then by image name, and nothing
// here needs either — every value is a reference under the platform's own repository.
func readImageSet(content io.ReadCloser, digestsPath string) ([]string, []string, error) {
	defer func() { _ = content.Close() }()

	var rawTags, rawDigests []byte

	reader := tar.NewReader(content)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("reading the installer's filesystem: %w", err)
		}

		switch header.Name {
		case imagesTagsFile:
			if rawTags, err = io.ReadAll(reader); err != nil {
				return nil, nil, fmt.Errorf("reading %s: %w", imagesTagsFile, err)
			}
		case digestsPath:
			if rawDigests, err = io.ReadAll(reader); err != nil {
				return nil, nil, fmt.Errorf("reading %s: %w", digestsPath, err)
			}
		}
	}

	tags, err := flatten(rawTags, imagesTagsFile)
	if err != nil {
		return nil, nil, err
	}
	digests, err := flatten(rawDigests, digestsPath)
	if err != nil {
		return nil, nil, err
	}
	return tags, digests, nil
}

// flatten turns a module-to-image-to-value mapping into the values it holds.
func flatten(raw []byte, what string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var byModule map[string]map[string]string
	if err := json.Unmarshal(raw, &byModule); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", what, err)
	}

	var values []string
	for _, images := range byModule {
		for _, value := range images {
			if value != "" {
				values = append(values, value)
			}
		}
	}
	return values, nil
}
