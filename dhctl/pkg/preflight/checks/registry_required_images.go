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

package checks

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/registryutil"
)

// requiredImagesSample is how many of the release's images are actually asked for.
//
// A few, not all: a release lists hundreds, and the question this answers is all-or-nothing. A
// registry populated with `d8 mirror push` holds every one of them; a registry populated by
// copying the tag with crane or skopeo — which is what people reach for — holds the Deckhouse
// image and none of the images it refers to. Five spread across the list settles that without
// several hundred round trips.
const requiredImagesSample = 5

// RegistryRequiredImagesCheck asks whether the registry holds the images the release is made of,
// and not just the tag that names it.
//
// A mirror with the tag and nothing behind it gets through deckhouse-image-available, and fails
// during bootstrap: the control plane never comes up, and what the operator sees is bashible
// looping on "072 etcd not running after 200s" inside the retry storm. In Unmanaged registry mode
// nothing checks this at all, because the in-cluster condition that would has no cluster to run in
// yet.
type RegistryRequiredImagesCheck struct {
	MetaConfig *config.MetaConfig

	// digests is the release's image list. Injected for the tests; nil means the one embedded in
	// this installer.
	digests func() (digests.ImagesDigests, error)
	// head asks the registry for one image without pulling it.
	head func(ref name.Reference, opts ...remote.Option) error
}

const RegistryRequiredImagesCheckName preflight.CheckName = "registry-required-images"

func (RegistryRequiredImagesCheck) Description() string {
	return "the registry holds the images this Deckhouse release is made of"
}

func (RegistryRequiredImagesCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (RegistryRequiredImagesCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c RegistryRequiredImagesCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("meta config is required")
	}

	all, err := c.imageDigests()
	if err != nil {
		// A build with no embedded digests cannot say what the release is made of. That is a
		// development installer, not a broken registry.
		return "", preflight.NotApplicable("this installer carries no image digests to look for")
	}

	sample := sampleDigests(all)
	if len(sample) == 0 {
		return "", preflight.NotApplicable("this installer carries no image digests to look for")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	repo := c.MetaConfig.Registry.Settings.ToModel().RemoteImagesRepo

	client, err := registryutil.NewRegistryClient(ctx, string(registry.Scheme), registry.CA)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField,
			Observed: err.Error(),
			Expected: "a PEM bundle the request can be made with",
			Fix:      "correct " + registryCAField,
		})
	}

	var missing []string
	for _, image := range sample {
		ref, err := parseImageReference(repo+"@"+image.digest, string(registry.Scheme))
		if err != nil {
			return "", fmt.Errorf("building a reference to %s: %w", image.name, err)
		}

		err = c.headImage(ref,
			remote.WithContext(ctx),
			remote.WithAuth(registryAuth(registry)),
			remote.WithTransport(client.Transport),
		)
		switch {
		case err == nil:
			continue
		case isImageAbsent(err):
			missing = append(missing, fmt.Sprintf("%s (%s)", image.name, shortDigest(image.digest)))
		default:
			// Anything that is not "no such image" is about the registry rather than about what
			// it holds, and registry-reachable owns that question.
			return "", &preflight.Failure{
				Checked:  fmt.Sprintf("the images of this release in %s", repo),
				Observed: classifyNetworkError(err),
				Expected: "the registry to answer",
				Fix:      fmt.Sprintf("check %s", registryImagesRepoField),
				Err:      err,
			}
		}
	}

	if len(missing) > 0 {
		return "", preflight.Permanent(&preflight.Failure{
			Checked: fmt.Sprintf("%d of the images this release is made of, in %s", len(sample), repo),
			Observed: fmt.Sprintf("%d of them are not there:\n- %s",
				len(missing), strings.Join(missing, "\n- ")),
			Expected: "every image of the release, which is what the Deckhouse image refers to",
			Fix: "mirror the release with `d8 mirror pull` and `d8 mirror push`. Copying the tag alone " +
				"(crane copy, skopeo copy) brings the Deckhouse image without the images it needs, " +
				"and the control plane never comes up",
		})
	}

	return fmt.Sprintf("%d sampled images of this release are present in %s", len(sample), repo), nil
}

func (c RegistryRequiredImagesCheck) imageDigests() (digests.ImagesDigests, error) {
	if c.digests != nil {
		return c.digests()
	}
	return digests.GetAllDigests()
}

func (c RegistryRequiredImagesCheck) headImage(ref name.Reference, opts ...remote.Option) error {
	if c.head != nil {
		return c.head(ref, opts...)
	}
	_, err := remote.Head(ref, opts...)
	return err
}

// sampledImage is one entry of the release's image list.
type sampledImage struct {
	name   string
	digest string
}

// sampleDigests picks up to requiredImagesSample images, spread evenly over the sorted list so
// the sample is the same on every run and does not sit in one section of the release.
func sampleDigests(all digests.ImagesDigests) []sampledImage {
	const n = requiredImagesSample

	var images []sampledImage
	for section, entries := range all {
		for entry, raw := range entries {
			digest, ok := raw.(string)
			if !ok || !strings.HasPrefix(digest, "sha256:") {
				continue
			}
			images = append(images, sampledImage{name: section + "/" + entry, digest: digest})
		}
	}

	sort.Slice(images, func(i, j int) bool { return images[i].name < images[j].name })
	if len(images) <= n {
		return images
	}

	sampled := make([]sampledImage, 0, n)
	step := len(images) / n
	for i := 0; i < n; i++ {
		sampled = append(sampled, images[i*step])
	}
	return sampled
}

// isImageAbsent reports whether the registry answered "no such image", as opposed to failing to
// answer at all.
func isImageAbsent(err error) bool {
	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		return false
	}
	for _, diagnostic := range transportErr.Errors {
		switch diagnostic.Code {
		case transport.ManifestUnknownErrorCode, transport.BlobUnknownErrorCode, transport.NameUnknownErrorCode:
			return true
		}
	}
	return false
}

// shortDigest keeps a message readable: the first twelve hex characters identify an image as well
// as all sixty-four do, and the reader is not going to type either.
func shortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) > 12 {
		digest = digest[:12]
	}
	return "sha256:" + digest
}

func RegistryRequiredImages(meta *config.MetaConfig) preflight.Check {
	check := RegistryRequiredImagesCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        RegistryRequiredImagesCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
