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
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	registry "github.com/deckhouse/deckhouse/pkg/registry"
	registryclient "github.com/deckhouse/deckhouse/pkg/registry/client"

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

// defaultRootHashInterval is how often the digest in force is re-read. A release
// changes it, and a release is not a thing that happens every minute; noticing one
// within half a minute is well inside the time a node takes to act on it anyway.
const defaultRootHashInterval = 30 * time.Second

// imageSource is the part of a registry client this needs: the config to read a
// label from, and the content to read a file from. Narrow on purpose — it is the
// seam the tests replace, and a wider one would have them implement operations
// nothing here performs.
type imageSource interface {
	GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)
	GetImage(ctx context.Context, tag string, opts ...registry.ImageGetOption) (registry.Image, error)
}

// rootHashResolver holds the answer for the digest currently in force, and the
// registry call that produces it.
//
// The render path only reads what is held: at any moment the cluster publishes
// exactly one OS image digest, so the hash is resolved when that digest appears and
// not while a node is being rendered. Keeping the registry out of the render is the
// point — a slow registry would otherwise delay every node's configuration, and a
// failing one would put a round trip into every pass.
//
// Failures leave the hash empty rather than propagating, and that is the design
// rather than laziness: the field it fills is optional, a node handed none reads the
// same value out of the artifact it downloads anyway, and failing instead would stop
// rendering configurations for every node in the cluster because a registry was
// briefly unreachable.
type rootHashResolver struct {
	// newSource is the registry client factory, replaced in tests.
	newSource func(reg *internalv1alpha1.Registry, repo string) (imageSource, error)

	mu     sync.Mutex
	digest string
	hash   string
}

func newRootHashResolver() *rootHashResolver {
	return &rootHashResolver{newSource: dialRegistry}
}

// known is what the render reads: the hash held for this digest, or "" for any
// other. Never dials anything — a digest this has not been told about yet is a node
// rendered without the field, which costs that node one download.
func (r *rootHashResolver) known(digest string) string {
	if digest == "" {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if digest != r.digest {
		return ""
	}
	return r.hash
}

// resolved reports whether the hash for this digest is already held, so the watcher
// can tell "nothing changed" from "changed and not yet read".
func (r *rootHashResolver) resolved(digest string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return digest != "" && digest == r.digest && r.hash != ""
}

// hold records the answer for a digest, replacing whatever the previous one was.
// Only one digest is ever held: the cluster publishes one, and remembering the old
// ones would be a cache of answers nothing will ask for again.
func (r *rootHashResolver) hold(digest, hash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.digest, r.hash = digest, hash
}

// refresh resolves the hash for digest unless it is already held, and is the only
// thing that talks to the registry.
func (r *rootHashResolver) refresh(ctx context.Context, reg *internalv1alpha1.Registry, repo, digest string) {
	if digest == "" || r.resolved(digest) {
		return
	}
	hash, err := r.lookup(ctx, reg, repo, digest)
	if err != nil {
		// Logged at info, not error: the node has its own way to the same answer, so
		// this is a lost optimisation and not a fault to page anyone about. The next
		// tick tries again.
		log.FromContext(ctx).Info("could not read the root hash of the OS image; nodes will read it from the artifact instead",
			"digest", digest, "error", err)
		return
	}
	r.hold(digest, hash)
	log.FromContext(ctx).Info("resolved the root hash of the OS image", "digest", digest, "rootHash", hash)
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
	// Insecure rather than the deprecated WithScheme, and it says the same thing:
	// the client treats "http" as insecure anyway, so the scheme this carries is
	// only ever the choice between plain HTTP and TLS.
	opts := []registryclient.Option{
		registryclient.WithInsecure(strings.EqualFold(reg.Scheme, "HTTP")),
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

// rootHashWatcher keeps the resolver in step with the digest the cluster publishes:
// it resolves at start and whenever that digest changes, so the render never has to.
//
// It polls rather than watches, and that is forced rather than chosen: the ConfigMap
// the digests live in is outside the manager's cache on purpose (see Setup), and a
// cached watch on ConfigMaps would mean caching every ConfigMap in the cluster to
// follow one. The object changes once per release, so a poll costs one Get per
// interval and notices a new release within it.
type rootHashWatcher struct {
	sources  *sourceReader
	resolver *rootHashResolver
	interval time.Duration
}

// NeedLeaderElection keeps this on every replica: it fills an in-memory value this
// process renders from, so a follower that ever becomes leader must already have it.
func (w *rootHashWatcher) NeedLeaderElection() bool { return false }

// Start satisfies manager.Runnable. The first pass runs immediately, so a controller
// that has just started renders with the hash rather than without it.
func (w *rootHashWatcher) Start(ctx context.Context) error {
	interval := w.interval
	if interval <= 0 {
		interval = defaultRootHashInterval
	}

	w.tick(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick reads what the cluster publishes and resolves the hash if that digest is new.
// Every failure is logged and dropped: this fills an optional field, and nothing here
// is worth taking the controller down for.
func (w *rootHashWatcher) tick(ctx context.Context) {
	logger := log.FromContext(ctx)

	images, err := w.sources.readImagesDigests(ctx)
	if err != nil {
		logger.Info("could not read the image digests while resolving the OS image root hash", "error", err)
		return
	}
	digest, err := digestAt(images, nodeManagerDigestsKey, osImageName)
	if err != nil {
		logger.Info("the release publishes no OS image digest yet", "error", err)
		return
	}
	if w.resolver.resolved(digest) {
		return
	}

	reg, imagesRepo, err := w.sources.readRegistry(ctx)
	if err != nil {
		logger.Info("could not read the registry while resolving the OS image root hash", "error", err)
		return
	}
	w.resolver.refresh(ctx, reg, imagesRepo, digest)
}
