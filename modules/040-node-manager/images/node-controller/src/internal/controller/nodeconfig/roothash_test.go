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

package nodeconfig

import (
	"context"
	"errors"
	"testing"

	registry "github.com/deckhouse/deckhouse/pkg/registry"
	registryclient "github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/deckhouse/deckhouse/pkg/registry/fake"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

const (
	testDigest   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	testRootHash = "695f8911cb5b6e65ae5d2ad4651b37c94d9a1eecf1c84a4ceccbb7375cdc63fb"
)

// stubSource is the registry as this resolver uses it, and it counts the calls: the
// point of half these tests is which of the two lookups ran, not only what came out.
type stubSource struct {
	config      *v1.ConfigFile
	image       v1.Image
	configErr   error
	imageErr    error
	configCalls int
	imageCalls  int
}

func (s *stubSource) GetImageConfig(_ context.Context, _ string) (*v1.ConfigFile, error) {
	s.configCalls++
	return s.config, s.configErr
}

func (s *stubSource) GetImage(_ context.Context, _ string, _ ...registry.ImageGetOption) (registry.Image, error) {
	s.imageCalls++
	if s.imageErr != nil {
		return nil, s.imageErr
	}
	return registryclient.NewImage(s.image, "test"), nil
}

func resolverFor(source imageSource) *rootHashResolver {
	return &rootHashResolver{
		newSource: func(*internalv1alpha1.Registry, string) (imageSource, error) { return source, nil },
		cache:     map[string]string{},
	}
}

func testRegistry() *internalv1alpha1.Registry {
	return &internalv1alpha1.Registry{Address: "registry.example.com", Path: "/deckhouse/ce", Scheme: "HTTPS"}
}

// The cheap path, and the one every release takes: the hash is a label on the image.
func TestTheRootHashIsReadFromTheLabel(t *testing.T) {
	source := &stubSource{config: &v1.ConfigFile{
		Config: v1.Config{Labels: map[string]string{rootHashLabel: testRootHash}},
	}}

	got := resolverFor(source).resolve(context.Background(), testRegistry(), "registry.example.com/deckhouse/ce", testDigest)
	if got != testRootHash {
		t.Fatalf("root hash = %q, want %q", got, testRootHash)
	}
	if source.imageCalls != 0 {
		t.Fatalf("fetched the image %d times, want the label to be enough", source.imageCalls)
	}
}

// An image from before the label existed still carries the file the label is stamped
// from, so the answer is the same and only the cost differs.
func TestWithoutALabelTheRootHashComesFromTheArtifact(t *testing.T) {
	image := fake.NewImageBuilder().
		WithFile("rootfs.erofs", "not the hash").
		WithFile("rootfs.roothash.p7s", "signature, not the hash").
		WithFile("rootfs.roothash", testRootHash).
		MustBuild()
	source := &stubSource{config: &v1.ConfigFile{}, image: image}

	got := resolverFor(source).resolve(context.Background(), testRegistry(), "registry.example.com/deckhouse/ce", testDigest)
	if got != testRootHash {
		t.Fatalf("root hash = %q, want %q", got, testRootHash)
	}
	if source.imageCalls != 1 {
		t.Fatalf("fetched the image %d times, want exactly one fallback", source.imageCalls)
	}
}

// The sibling signature file must not be mistaken for the hash: it sits next to it
// under a name the same prefix match would accept.
func TestTheSignatureSiblingIsNotMistakenForTheRootHash(t *testing.T) {
	image := fake.NewImageBuilder().WithFile("./rootfs.roothash.p7s", "signature").MustBuild()
	source := &stubSource{config: &v1.ConfigFile{}, image: image}

	if got := resolverFor(source).resolve(context.Background(), testRegistry(), "", testDigest); got != "" {
		t.Fatalf("root hash = %q, want none found", got)
	}
}

// A registry that cannot be read costs a node one download, not a configuration: the
// field is optional and the node reads the same value out of the artifact.
func TestAFailedLookupYieldsNoHashAndNoError(t *testing.T) {
	source := &stubSource{configErr: errors.New("registry unreachable"), imageErr: errors.New("registry unreachable")}

	if got := resolverFor(source).resolve(context.Background(), testRegistry(), "", testDigest); got != "" {
		t.Fatalf("root hash = %q, want none on a failed lookup", got)
	}
}

// The mapping is immutable, so it is looked up once per image however many nodes ask.
func TestTheRootHashIsLookedUpOncePerImage(t *testing.T) {
	source := &stubSource{config: &v1.ConfigFile{
		Config: v1.Config{Labels: map[string]string{rootHashLabel: testRootHash}},
	}}
	resolver := resolverFor(source)

	for range 3 {
		if got := resolver.resolve(context.Background(), testRegistry(), "", testDigest); got != testRootHash {
			t.Fatalf("root hash = %q, want %q", got, testRootHash)
		}
	}
	if source.configCalls != 1 {
		t.Fatalf("read the image config %d times, want one lookup for one image", source.configCalls)
	}
}

// The path the client is given has to be the repository inside the registry, with the
// host taken off: the client holds the host separately and would otherwise address
// registry.example.com/registry.example.com/...
func TestRepositorySegmentsStripTheHost(t *testing.T) {
	for _, tc := range []struct {
		name, address, repo, path string
		want                      []string
	}{
		{"images repo", "registry.example.com", "registry.example.com/deckhouse/ce", "/deckhouse/ce", []string{"deckhouse", "ce"}},
		{"falls back to the registry path", "registry.example.com", "", "/deckhouse/fe", []string{"deckhouse", "fe"}},
		{"no path at all", "registry.example.com", "registry.example.com", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := repositorySegments(tc.address, tc.repo, tc.path)
			if len(got) != len(tc.want) {
				t.Fatalf("segments = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("segments = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
