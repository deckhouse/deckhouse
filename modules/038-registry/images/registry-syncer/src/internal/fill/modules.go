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

// modulesRepository is where a module's package lives under the platform's own repository:
// `<repository>/modules/<name>`, tagged with the module's version.
const modulesRepository = "modules"

// moduleDigestsFile is how a module package declares the images it consists of.
//
// The same file the controller reads to decide what a module runs
// (deckhouse-controller/internal/packages/loader, `digestsFile`), and it is flat — image name to
// digest — where the platform's own account of its set is nested by module. That difference is the
// only reason this is not the same parser.
const moduleDigestsFile = "images_digests.json"

// ModuleRef is a module the cluster keeps, at the version it keeps.
type ModuleRef struct {
	Name    string
	Version string
}

// ModuleReferences enumerates what the modules a cluster keeps consist of.
//
// This exists because the platform's release does not account for them, and completeness that ignores
// them is completeness in name only. A release declares the platform's images — 333 of them on a
// measured cluster — while the modules the cluster actually runs (ingress-nginx, prometheus, upmeter
// and the rest) are packaged separately and declared separately. A store judged complete on the
// platform's set alone can be missing every one of them, and "complete" is precisely the answer that
// authorizes cutting the cluster off from its upstream: measured on a bundle, 474 of 474 platform
// manifests present and 34 module images the cluster was running absent.
//
// Read out of each module's own package, for the same reason the platform's set is read out of the
// image the cluster runs: it is an account the module gives of itself, so nothing has to be inferred
// from what a registry happens to hold, and nothing has to be permitted beyond pulling what the
// cluster already pulls.
//
// A module whose package cannot be read is an error rather than a module quietly left out. Leaving it
// out would lower the bar for completeness by exactly the images nobody could account for, which is
// the failure this whole function exists to prevent.
func ModuleReferences(
	ctx context.Context, source Registry, puller *remote.Puller, modules []ModuleRef,
) ([]name.Reference, error) {
	if len(modules) == 0 {
		return nil, nil
	}

	registry, err := parseRegistry(source)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var references []name.Reference
	add := func(reference name.Reference) {
		if _, already := seen[reference.String()]; already {
			return
		}
		seen[reference.String()] = struct{}{}
		references = append(references, reference)
	}

	for _, module := range modules {
		if module.Name == "" || module.Version == "" {
			continue
		}

		repository := registry.Repo(source.Repository, modulesRepository, module.Name)

		// The package itself, by tag: it is an image like any other, and a cluster that cannot pull it
		// cannot reinstall the module.
		packaged := repository.Tag(module.Version)
		add(packaged)

		digests, err := moduleImageSet(ctx, puller, packaged)
		if err != nil {
			return nil, fmt.Errorf("reading the image set of module %s: %w", module.Name, err)
		}

		for _, digest := range digests {
			parsed, err := v1.NewHash(digest)
			if err != nil {
				return nil, fmt.Errorf("module %s declares %q, which is not a digest: %w",
					module.Name, digest, err)
			}
			add(repository.Digest(parsed.String()))
		}
	}

	return references, nil
}

// moduleImageSet reads the digests a module package declares.
func moduleImageSet(
	ctx context.Context, puller *remote.Puller, reference name.Reference,
) ([]string, error) {
	descriptor, err := puller.Get(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", reference, err)
	}
	image, err := descriptor.Image()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", reference, err)
	}

	digests, err := readModuleDigests(mutate.Extract(image))
	if err != nil {
		return nil, fmt.Errorf("reading the image set out of %s: %w", reference, err)
	}

	return digests, nil
}

// readModuleDigests pulls the digest map out of a flattened module package.
//
// An absent or empty file is not an error: a module can legitimately consist of no images of its own —
// several do, being nothing but templates and hooks that run in the platform's own image.
func readModuleDigests(content io.ReadCloser) ([]string, error) {
	defer func() { _ = content.Close() }()

	var raw []byte

	reader := tar.NewReader(content)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the package filesystem: %w", err)
		}
		if header.Name != moduleDigestsFile {
			continue
		}
		if raw, err = io.ReadAll(reader); err != nil {
			return nil, fmt.Errorf("reading %s: %w", moduleDigestsFile, err)
		}
	}

	if len(raw) == 0 {
		return nil, nil
	}

	var byImage map[string]string
	if err := json.Unmarshal(raw, &byImage); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", moduleDigestsFile, err)
	}

	digests := make([]string, 0, len(byImage))
	for _, digest := range byImage {
		if digest != "" {
			digests = append(digests, digest)
		}
	}

	return digests, nil
}
