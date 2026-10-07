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

package oci

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"testing/fstest"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/types"
)

// layout is an OCI image layout in memory, built the way `d8 mirror pull` writes the d8 CLI image: one
// tagged image index listing a platform image and a buildx attestation manifest.
type layout struct {
	fs          fstest.MapFS
	index       ociv1.Descriptor
	platform    ociv1.Descriptor
	attestation ociv1.Descriptor
}

func (l *layout) blob(content []byte, mediaType string) ociv1.Descriptor {
	dgst := digest.FromBytes(content)
	l.fs["blobs/"+dgst.Algorithm().String()+"/"+dgst.Encoded()] = &fstest.MapFile{Data: content}
	return ociv1.Descriptor{MediaType: mediaType, Digest: dgst, Size: int64(len(content))}
}

func (l *layout) image(layer string) ociv1.Descriptor {
	config := l.blob([]byte(`{"architecture":"amd64","os":"linux"}`+layer), ociv1.MediaTypeImageConfig)
	data := l.blob([]byte(layer), ociv1.MediaTypeImageLayerGzip)
	manifest, _ := json.Marshal(ociv1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ociv1.MediaTypeImageManifest,
		Config:    config,
		Layers:    []ociv1.Descriptor{data},
	})
	return l.blob(manifest, ociv1.MediaTypeImageManifest)
}

func newCLILayout(t *testing.T) *layout {
	t.Helper()

	l := &layout{fs: fstest.MapFS{
		ociv1.ImageLayoutFile: &fstest.MapFile{Data: []byte(`{"imageLayoutVersion":"1.0.0"}`)},
	}}

	l.platform = l.image("d8 binary")
	l.platform.Platform = &ociv1.Platform{Architecture: "amd64", OS: "linux"}
	l.attestation = l.image("provenance")
	l.attestation.Platform = &ociv1.Platform{Architecture: "unknown", OS: "unknown"}
	l.attestation.Annotations = map[string]string{
		referenceTypeAnnotation:       attestationManifest,
		"vnd.docker.reference.digest": l.platform.Digest.String(),
	}

	index, _ := json.Marshal(ociv1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ociv1.MediaTypeImageIndex,
		Manifests: []ociv1.Descriptor{l.platform, l.attestation},
	})
	l.index = l.blob(index, ociv1.MediaTypeImageIndex)
	l.index.Annotations = map[string]string{types.ShortTagAnnotation: "v0.13.1"}

	top, _ := json.Marshal(ociv1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ociv1.MediaTypeImageIndex,
		Manifests: []ociv1.Descriptor{l.index},
	})
	l.fs[ociv1.ImageIndexFile] = &fstest.MapFile{Data: top}
	return l
}

func (l *layout) remove(desc ociv1.Descriptor) {
	delete(l.fs, "blobs/"+desc.Digest.Algorithm().String()+"/"+desc.Digest.Encoded())
}

func TestValidateLayoutAcceptsAnImageIndex(t *testing.T) {
	l := newCLILayout(t)
	if err := ValidateLayout(context.Background(), l.fs); err != nil {
		t.Fatalf("an image index with every image present was refused: %v", err)
	}
}

func TestValidateLayoutToleratesAMissingAttestation(t *testing.T) {
	l := newCLILayout(t)
	l.remove(l.attestation)
	if err := ValidateLayout(context.Background(), l.fs); err != nil {
		t.Fatalf("a mirror that left the attestation behind still serves the image whole: %v", err)
	}
}

func TestValidateLayoutRefusesAMissingPlatformImage(t *testing.T) {
	l := newCLILayout(t)
	l.remove(l.platform)
	if err := ValidateLayout(context.Background(), l.fs); err == nil {
		t.Fatal("an index whose platform image is absent cannot be pulled, and was accepted")
	}
}

func TestValidateLayoutRefusesAMissingLayerBehindAnIndex(t *testing.T) {
	l := newCLILayout(t)
	l.remove(l.blob([]byte("d8 binary"), ociv1.MediaTypeImageLayerGzip))
	if err := ValidateLayout(context.Background(), l.fs); err == nil {
		t.Fatal("a layer missing under the platform image was not noticed through the index")
	}
}

// A client pulls the tag, reads the index, and asks for its platform's image by digest. That second
// request is the one an index in index.json alone could not answer.
func TestLayoutStoreServesTheImagesAnIndexLists(t *testing.T) {
	l := newCLILayout(t)
	st, err := NewLayoutStore(l.fs)
	if err != nil {
		t.Fatalf("NewLayoutStore: %v", err)
	}
	ctx := context.Background()

	desc, rc, err := st.Resolve(ctx, "v0.13.1")
	if err != nil {
		t.Fatalf("resolving the tag: %v", err)
	}
	_ = rc.Close()
	if desc.Digest != l.index.Digest || desc.MediaType != ociv1.MediaTypeImageIndex {
		t.Fatalf("the tag resolved to %v, want the index %s", desc, l.index.Digest)
	}

	desc, rc, err = st.Resolve(ctx, l.platform.Digest.String())
	if err != nil {
		t.Fatalf("resolving the platform image by digest: %v", err)
	}
	body, _ := io.ReadAll(rc)
	_ = rc.Close()
	if desc.MediaType != ociv1.MediaTypeImageManifest || digest.FromBytes(body) != l.platform.Digest {
		t.Fatalf("the platform image came back as %v", desc)
	}
}

func TestLayoutStoreLeavesOutChildrenTheLayoutDoesNotHold(t *testing.T) {
	l := newCLILayout(t)
	l.remove(l.attestation)
	st, err := NewLayoutStore(l.fs)
	if err != nil {
		t.Fatalf("NewLayoutStore: %v", err)
	}
	if _, _, err := st.Resolve(context.Background(), l.attestation.Digest.String()); err == nil {
		t.Fatal("an attestation that is not in the layout was advertised as resolvable")
	}
}
