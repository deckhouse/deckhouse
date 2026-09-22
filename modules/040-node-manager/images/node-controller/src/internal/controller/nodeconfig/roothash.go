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
	"archive/tar"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	registry "github.com/deckhouse/deckhouse/pkg/registry"
	registryclient "github.com/deckhouse/deckhouse/pkg/registry/client"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// The root hash of the OS image: what it is for, and why the cluster is what looks
// it up.
//
// A node decides whether to replace its root by comparing what the cluster asks for
// with what it runs. The digest cannot answer that — it names the packaging, and the
// same root published again arrives under a new one, so a node comparing digests
// goes off to update itself onto a copy of itself: a download and a reboot per node,
// per republish. The dm-verity root hash names the content, and the node always
// knows its own.
//
// The lookup lives here because this is the side with the registry: a node reaches
// it through the packages proxy, which serves artifacts and does not serve
// manifests. And because the mapping digest -> root hash is immutable, it is work
// per image rather than per node.
const (
	// rootHashLabel is where the engine build records it, on the release tags of
	// the OS image. See engine/docs/os-image-identity.md.
	rootHashLabel = "io.deckhouse.engine.rootfs.roothash"
	// rootHashMember is the same value inside the artifact, which is what the node
	// falls back to and what this falls back to: an image built before the label
	// existed still carries the file.
	rootHashMember = "rootfs.roothash"
)

// imageSource is the part of a registry client this needs: the config to read a
// label from, and the content to read a file from. Narrow on purpose — it is the
// seam the tests replace, and a wider one would have them implement operations
// nothing here performs.
type imageSource interface {
	GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)
	GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error)
}

// rootHashResolver turns an OS image digest into the root hash of the root inside
// it, once per image.
//
// Every failure here returns an empty hash rather than an error, and that is the
// design rather than laziness: the field it fills is optional, its absence costs a
// node one download of an artifact it can read the same value from, and failing the
// pass instead would stop rendering configurations for every node in the cluster
// because a registry was briefly unreachable.
type rootHashResolver struct {
	// newSource is the registry client factory, replaced in tests.
	newSource func(reg *internalv1alpha1.Registry, repo string) (imageSource, error)

	mu    sync.Mutex
	cache map[string]string
}

func newRootHashResolver() *rootHashResolver {
	return &rootHashResolver{newSource: dialRegistry, cache: map[string]string{}}
}

// resolve is the whole entry point: a hash, or "" with the reason logged.
func (r *rootHashResolver) resolve(ctx context.Context, reg *internalv1alpha1.Registry, repo, digest string) string {
	if digest == "" {
		return ""
	}
	if hash, ok := r.cached(digest); ok {
		return hash
	}

	hash, err := r.lookup(ctx, reg, repo, digest)
	if err != nil {
		// Logged at info, not error: the node has its own way to the same answer, so
		// this is a lost optimisation and not a fault to page anyone about.
		log.FromContext(ctx).Info("could not read the root hash of the OS image; nodes will read it from the artifact instead",
			"digest", digest, "error", err)
		return ""
	}

	r.remember(digest, hash)
	return hash
}

func (r *rootHashResolver) cached(digest string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	hash, ok := r.cache[digest]
	return hash, ok
}

func (r *rootHashResolver) remember(digest, hash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	r.cache[digest] = hash
}

// lookup reads the label, and reads the artifact when there is no label. Both
// answers are the same value by construction — the label is stamped from that very
// file — so which one arrives does not matter to the node; only what it cost.
func (r *rootHashResolver) lookup(ctx context.Context, reg *internalv1alpha1.Registry, repo, digest string) (string, error) {
	if reg == nil {
		return "", errors.New("the cluster registry is not known yet")
	}
	source, err := r.newSource(reg, repo)
	if err != nil {
		return "", err
	}

	config, err := source.GetImageConfig(ctx, digest)
	if err != nil {
		return "", fmt.Errorf("read the image config: %w", err)
	}
	if config != nil {
		if hash := strings.TrimSpace(config.Config.Labels[rootHashLabel]); hash != "" {
			return hash, nil
		}
	}

	// No label: an image from before it was published, or one published outside the
	// release flow. The file is still there, and reading it is what the node would
	// do anyway — done once here instead of once per node.
	image, err := source.GetImage(ctx, digest)
	if err != nil {
		return "", fmt.Errorf("fetch the image: %w", err)
	}
	content := image.Extract()
	defer content.Close()

	hash, err := rootHashFromContent(content)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// rootHashFromContent finds the root hash file in the image's flattened content.
// Matched by base name: the members arrive with the layer's own prefixes ("./" on
// some builds, none on others), and the sibling rootfs.roothash.p7s must not be
// mistaken for it.
func rootHashFromContent(content io.Reader) (string, error) {
	reader := tar.NewReader(content)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("the image carries no %s and no %s label", rootHashMember, rootHashLabel)
		}
		if err != nil {
			return "", fmt.Errorf("read the image content: %w", err)
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != rootHashMember {
			continue
		}
		// The file is 64 hex characters written without a trailing newline, so a
		// bounded read is not a limit anyone can reach with a valid one.
		raw, err := io.ReadAll(io.LimitReader(reader, 1024))
		if err != nil {
			return "", fmt.Errorf("read %s from the image: %w", rootHashMember, err)
		}
		hash := strings.TrimSpace(string(raw))
		if hash == "" {
			return "", fmt.Errorf("%s in the image is empty", rootHashMember)
		}
		return hash, nil
	}
}

// dialRegistry builds the client from what the cluster's registry secret already
// gave the reader: the same host, path, scheme, CA and credentials a node is handed.
func dialRegistry(reg *internalv1alpha1.Registry, repo string) (imageSource, error) {
	opts := []registryclient.Option{
		registryclient.WithScheme(strings.ToLower(reg.Scheme)),
	}
	if reg.CA != "" {
		opts = append(opts, registryclient.WithCA(reg.CA))
	}
	if reg.Auth != "" {
		user, password, err := decodeAuth(reg.Auth)
		if err != nil {
			return nil, err
		}
		opts = append(opts, registryclient.WithLoginPassword(user, password))
	}

	client := registryclient.New(reg.Address, opts...)
	// repo is the registry host plus the repository path; the client takes the host
	// on its own and the path as segments.
	if segments := repositorySegments(reg.Address, repo, reg.Path); len(segments) > 0 {
		return client.WithSegment(segments...), nil
	}
	return client, nil
}

// repositorySegments is the path part of the repository the release lives in,
// preferring what the secret said the images repository is and falling back to the
// registry's own path.
func repositorySegments(address, repo, registryPath string) []string {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(repo, address), "/")
	if trimmed == "" {
		trimmed = strings.Trim(registryPath, "/")
	}
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// decodeAuth splits the base64 "user:password" a docker config carries.
func decodeAuth(auth string) (string, string, error) {
	raw, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", fmt.Errorf("decode the registry credentials: %w", err)
	}
	user, password, ok := strings.Cut(string(raw), ":")
	if !ok {
		return "", "", errors.New("the registry credentials are not user:password")
	}
	return user, password, nil
}
