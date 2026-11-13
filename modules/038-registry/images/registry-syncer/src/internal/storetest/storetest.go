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

// Package storetest lays out a registry store on disk the way distribution does, for tests: blobs by
// their real sha256, manifests linked from repositories.
package storetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// New is an empty store.
func New(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// Digest is the sha256 digest of content.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// BlobPath is where the store keeps a blob's content.
func BlobPath(root, digest string) string {
	algorithm, encoded, _ := strings.Cut(digest, ":")
	return filepath.Join(root, "docker", "registry", "v2", "blobs", algorithm, encoded[:2], encoded, "data")
}

// Blob writes content as a blob and returns its digest.
func Blob(t *testing.T, root string, content []byte) string {
	t.Helper()
	digest := Digest(content)
	path := BlobPath(root, digest)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return digest
}

// Corrupt overwrites a blob's content with something that does not match its digest.
func Corrupt(t *testing.T, root, digest string) {
	t.Helper()
	if err := os.WriteFile(BlobPath(root, digest), []byte("bit rot of "+digest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Remove deletes a blob's content.
func Remove(t *testing.T, root, digest string) {
	t.Helper()
	if err := os.Remove(BlobPath(root, digest)); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, path, digest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(digest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func repositoryDir(root, repository string) string {
	return filepath.Join(root, "docker", "registry", "v2", "repositories", filepath.FromSlash(repository))
}

// Revision links a manifest from a repository.
func Revision(t *testing.T, root, repository, digest string) {
	t.Helper()
	algorithm, encoded, _ := strings.Cut(digest, ":")
	link(t, filepath.Join(repositoryDir(root, repository), "_manifests", "revisions", algorithm, encoded, "link"), digest)
}

// Layer links a blob from a repository.
func Layer(t *testing.T, root, repository, digest string) {
	t.Helper()
	algorithm, encoded, _ := strings.Cut(digest, ":")
	link(t, filepath.Join(repositoryDir(root, repository), "_layers", algorithm, encoded, "link"), digest)
}

type descriptor struct {
	MediaType string `json:"mediaType,omitempty"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size,omitempty"`
}

// ManifestContent is an image manifest naming config and layers.
func ManifestContent(config string, layers ...string) []byte {
	body := struct {
		SchemaVersion int          `json:"schemaVersion"`
		MediaType     string       `json:"mediaType"`
		Config        descriptor   `json:"config"`
		Layers        []descriptor `json:"layers"`
	}{SchemaVersion: 2, MediaType: "application/vnd.oci.image.manifest.v1+json", Config: descriptor{Digest: config}}
	for _, layer := range layers {
		body.Layers = append(body.Layers, descriptor{Digest: layer})
	}
	content, _ := json.Marshal(body)
	return content
}

// Image writes an image of the given layers — each a blob of its own content — with a config,
// links it from repository, and returns the manifest's digest and the layers' digests.
func Image(t *testing.T, root, repository string, layers ...string) (string, []string) {
	t.Helper()
	config := Blob(t, root, []byte(`{"config":"`+repository+strings.Join(layers, ",")+`"}`))
	Layer(t, root, repository, config)
	digests := make([]string, 0, len(layers))
	for _, layer := range layers {
		digest := Blob(t, root, []byte(layer))
		Layer(t, root, repository, digest)
		digests = append(digests, digest)
	}
	manifest := Blob(t, root, ManifestContent(config, digests...))
	Revision(t, root, repository, manifest)
	return manifest, append([]string{config}, digests...)
}

// Index writes an image index of the given manifests, links it from repository, and returns its digest.
func Index(t *testing.T, root, repository string, children ...string) string {
	t.Helper()
	body := struct {
		SchemaVersion int          `json:"schemaVersion"`
		MediaType     string       `json:"mediaType"`
		Manifests     []descriptor `json:"manifests"`
	}{SchemaVersion: 2, MediaType: "application/vnd.oci.image.index.v1+json"}
	for _, child := range children {
		body.Manifests = append(body.Manifests, descriptor{Digest: child})
	}
	content, _ := json.Marshal(body)
	digest := Blob(t, root, content)
	Revision(t, root, repository, digest)
	return digest
}
