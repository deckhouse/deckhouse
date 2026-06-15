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

// Package fake provides in-memory implementations of the registry client and
// related types for use in tests.  The key types are:
//
//   - [ImageBuilder]   – fluent builder that creates a v1.Image from text files and OCI metadata.
//   - [Registry]       – an in-memory registry scoped to a single host (e.g. "gcr.io").
//   - [Client]         – implements [localreg.Client] and dispatches every call to the
//     correct [Registry] instance based on the URL path.
package fake

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// fileEntry is a single file to embed inside an image layer.
type fileEntry struct {
	path    string
	content []byte
}

// ImageBuilder is a fluent builder that assembles a v1.Image from a set of
// plain-text files and optional OCI metadata (labels, platform, …).
//
// Usage:
//
//	img, err := fake.NewImageBuilder().
//	    WithFile("version.json", `{"version":"v1.2.3"}`).
//	    WithFile("changelog.yaml", changelogYAML).
//	    WithLabel("org.opencontainers.image.version", "v1.2.3").
//	    WithPlatform("linux", "amd64").
//	    Build()
type ImageBuilder struct {
	files      []fileEntry
	labels     map[string]string
	cfg        *v1.ConfigFile // optional; merged on top of defaults when set
	os         string
	arch       string
	variant    string
	env        []string
	entrypoint []string
	cmd        []string
	workingDir string
}

// NewImageBuilder returns a new, empty [ImageBuilder].
func NewImageBuilder() *ImageBuilder {
	return &ImageBuilder{
		labels: make(map[string]string),
		os:     "linux",
		arch:   "amd64",
	}
}

// WithFile adds (or replaces) a file at path with the given string content.
// The path is used as-is inside the tar archive.
func (b *ImageBuilder) WithFile(path, content string) *ImageBuilder {
	for i, f := range b.files {
		if f.path == path {
			b.files[i].content = []byte(content)
			return b
		}
	}
	b.files = append(b.files, fileEntry{path: path, content: []byte(content)})
	return b
}

// WithLabel attaches an OCI annotation / Docker label key=value pair to the
// image config.
func (b *ImageBuilder) WithLabel(key, value string) *ImageBuilder {
	b.labels[key] = value
	return b
}

// WithPlatform sets the OS and architecture for the image.
// Defaults are "linux" and "amd64" when not set.
func (b *ImageBuilder) WithPlatform(os, arch string) *ImageBuilder {
	b.os = os
	b.arch = arch
	return b
}

// WithVariant sets the platform variant (e.g. "v7" for ARM).
func (b *ImageBuilder) WithVariant(variant string) *ImageBuilder {
	b.variant = variant
	return b
}

// WithEnv adds environment variables to the image config.
// Each entry should be in "KEY=VALUE" format.
func (b *ImageBuilder) WithEnv(env ...string) *ImageBuilder {
	b.env = append(b.env, env...)
	return b
}

// WithEntrypoint sets the image entrypoint.
func (b *ImageBuilder) WithEntrypoint(entrypoint ...string) *ImageBuilder {
	b.entrypoint = entrypoint
	return b
}

// WithCmd sets the image default command.
func (b *ImageBuilder) WithCmd(cmd ...string) *ImageBuilder {
	b.cmd = cmd
	return b
}

// WithWorkingDir sets the default working directory for the image.
func (b *ImageBuilder) WithWorkingDir(dir string) *ImageBuilder {
	b.workingDir = dir
	return b
}

// WithConfig completely replaces the base v1.ConfigFile.  Labels set via
// [WithLabel] are still merged on top of this config at [Build] time.
func (b *ImageBuilder) WithConfig(cfg *v1.ConfigFile) *ImageBuilder {
	b.cfg = cfg
	return b
}

// Build assembles and returns a v1.Image.  The image contains exactly one
// gzipped-tar layer that holds all files registered with [WithFile].
func (b *ImageBuilder) Build() (v1.Image, error) {
	layer, err := b.buildLayer()
	if err != nil {
		return nil, fmt.Errorf("fake: build layer: %w", err)
	}

	base, err := mutate.AppendLayers(empty.Image, layer)
	if err != nil {
		return nil, fmt.Errorf("fake: append layer: %w", err)
	}

	cf, err := base.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("fake: get config file: %w", err)
	}

	if b.cfg != nil {
		cf = deepCopyConfigFile(b.cfg)
	}

	// Apply platform fields.
	if b.os != "" {
		cf.OS = b.os
	}
	if b.arch != "" {
		cf.Architecture = b.arch
	}
	if b.variant != "" {
		cf.Variant = b.variant
	}

	// Apply run-config fields.
	if len(b.env) > 0 {
		cf.Config.Env = append(cf.Config.Env, b.env...)
	}
	if len(b.entrypoint) > 0 {
		cf.Config.Entrypoint = b.entrypoint
	}
	if len(b.cmd) > 0 {
		cf.Config.Cmd = b.cmd
	}
	if b.workingDir != "" {
		cf.Config.WorkingDir = b.workingDir
	}

	if cf.Config.Labels == nil {
		cf.Config.Labels = make(map[string]string)
	}
	for k, v := range b.labels {
		cf.Config.Labels[k] = v
	}

	img, err := mutate.ConfigFile(base, cf)
	if err != nil {
		return nil, fmt.Errorf("fake: set config file: %w", err)
	}

	return img, nil
}

// MustBuild is like [Build] but panics on error.  Useful in test
// initialisation code where propagating an error is inconvenient.
func (b *ImageBuilder) MustBuild() v1.Image {
	img, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("fake.ImageBuilder.MustBuild: %v", err))
	}
	return img
}

// buildLayer creates a gzipped-tar layer from the registered files.
func (b *ImageBuilder) buildLayer() (v1.Layer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	now := time.Now()

	for _, fe := range b.files {
		hdr := &tar.Header{
			Name:     fe.path,
			Mode:     0644,
			Size:     int64(len(fe.content)),
			Typeflag: tar.TypeReg,
			ModTime:  now,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(fe.content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	compressed := buf.Bytes()

	// Verify digest computation works (tarball package derives it internally).
	h := sha256.Sum256(compressed)
	if _, hashErr := v1.NewHash(fmt.Sprintf("sha256:%x", h)); hashErr != nil {
		return nil, hashErr
	}

	opener := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(compressed)), nil
	}
	return tarball.LayerFromOpener(opener,
		tarball.WithMediaType(types.DockerLayer),
		tarball.WithCompressedCaching,
	)
}

// deepCopyConfigFile returns a shallow copy of cf with a deep-copied Labels map.
func deepCopyConfigFile(src *v1.ConfigFile) *v1.ConfigFile {
	if src == nil {
		return &v1.ConfigFile{}
	}
	cp := *src
	if src.Config.Labels != nil {
		cp.Config.Labels = make(map[string]string, len(src.Config.Labels))
		for k, v := range src.Config.Labels {
			cp.Config.Labels[k] = v
		}
	}
	return &cp
}

// ExtractFiles reads all regular files from an image's flattened filesystem
// and returns them as a map of path → content string.  Test helper.
func ExtractFiles(img v1.Image) (map[string]string, error) {
	rc := mutate.Extract(img)
	defer rc.Close()

	result := make(map[string]string)

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fake: extract: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("fake: read file %q: %w", hdr.Name, err)
		}
		result[hdr.Name] = string(data)
	}
	return result, nil
}

// MustMarshalJSON marshals v to a compact JSON string.  Panics on error.
// Useful for building JSON file content in tests, e.g.:
//
//	fake.NewImageBuilder().WithFile("data.json", fake.MustMarshalJSON(myStruct))
func MustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("fake.MustMarshalJSON: %v", err))
	}
	return string(b)
}
