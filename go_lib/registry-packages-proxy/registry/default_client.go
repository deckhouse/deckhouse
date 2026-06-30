// Copyright 2024 Flant JSC
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

package registry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"

	ddk "github.com/deckhouse/delivery-kit-sdk/pkg/signature/image"
	"github.com/deckhouse/rootca"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
)

type DefaultClient struct{}

func (c *DefaultClient) GetPackage(ctx context.Context, log log.Logger, config *ClientConfig, digest string, path string) (int64, string, io.ReadCloser, error) {
	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	repository, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return 0, "", nil, err
	}

	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return 0, "", nil, err
	}

	image, err := remote.Image(
		repository.Digest(digest),
		remoteOpts...)

	if err != nil {
		e := &transport.Error{}
		if errors.As(err, &e) {
			log.Error(e.Error())
			if e.StatusCode == http.StatusNotFound {
				return 0, "", nil, ErrPackageNotFound
			}
		}
		return 0, "", nil, err
	}

	manifest, err := image.Manifest()
	if err != nil {
		return 0, "", nil, err
	}

	// Verify image signature
	if config.SignCheck {
		log.Infof("verify image signature: %s %s", path, digest)
		if err := ddk.VerifyImageManifestSignature(ctx, []string{rootca.RootCABase64}, manifest); err != nil {
			log.Error("verify image signature failed: %w", err)
		}
	}

	layer, err := selectImageLayer(image, config.FlattenLayers)
	if err != nil {
		return 0, "", nil, err
	}

	size, err := layer.Size()
	if err != nil {
		return 0, "", nil, err
	}

	hash, err := layer.Digest()
	if err != nil {
		return 0, "", nil, err
	}

	reader, err := layer.Compressed()
	if err != nil {
		return 0, "", nil, err
	}

	return size, hash.Hex, reader, nil
}

// selectImageLayer returns the bytes-bearing layer requested by the caller:
// either the last layer of the image (legacy behavior) or a synthetic layer
// that flattens all layers into one filesystem (FlattenLayers=true). The
// flattened path is needed for images whose interesting file may live in any
// layer, e.g. icon extraction; the last-layer path preserves the historical
// /package and rpp-get contract.
func selectImageLayer(image v1.Image, flatten bool) (v1.Layer, error) {
	if !flatten {
		layers, err := image.Layers()
		if err != nil {
			return nil, err
		}
		if len(layers) == 0 {
			return nil, fmt.Errorf("image has no layers")
		}
		return layers[len(layers)-1], nil
	}

	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return mutate.Extract(image), nil
	})
}

func (c *DefaultClient) ResolveTag(ctx context.Context, log log.Logger, config *ClientConfig, path string, tag string, platform *v1.Platform) (string, error) {
	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	repository, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return "", err
	}

	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return "", err
	}

	desc, err := remote.Get(repository.Tag(tag), remoteOpts...)
	if err != nil {
		e := &transport.Error{}
		if errors.As(err, &e) {
			log.Error(e.Error())
			if e.StatusCode == http.StatusNotFound {
				return "", ErrPackageNotFound
			}
		}
		return "", err
	}

	// For a multi-platform image index, resolve to the per-platform child manifest
	// digest. The downstream cache is keyed by manifest digest, so returning the
	// shared index digest would make platforms collide on one cache entry.
	if platform != nil && desc.MediaType.IsIndex() {
		idx, err := desc.ImageIndex()
		if err != nil {
			return "", err
		}

		return childDigestForPlatform(idx, platform)
	}

	return desc.Digest.String(), nil
}

// childDigestForPlatform returns the digest of the index child manifest matching
// platform.
//
// Walking the index manually (instead of remote.Image + WithPlatform)
// lets us return ErrPackageNotFound for an absent platform, which the handler maps
// to a clean 404 rather than a generic error.
func childDigestForPlatform(idx v1.ImageIndex, platform *v1.Platform) (string, error) {
	manifest, err := idx.IndexManifest()
	if err != nil {
		return "", err
	}

	for _, m := range manifest.Manifests {
		if m.Platform == nil {
			continue
		}
		if m.Platform.OS != platform.OS || m.Platform.Architecture != platform.Architecture {
			continue
		}
		if platform.Variant != "" && m.Platform.Variant != platform.Variant {
			continue
		}

		return m.Digest.String(), nil
	}

	return "", ErrPackageNotFound
}

// GetRawManifest returns the raw manifest bytes and media type for path:ref without
// pulling layers. ref is a tag or a digest. remote.Get fetches only the manifest, so
// this is cheap; the caller (the CLI) parses whatever it needs from the bytes.
func (c *DefaultClient) GetRawManifest(ctx context.Context, log log.Logger, config *ClientConfig, path string, ref string) ([]byte, string, error) {
	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	repository, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return nil, "", err
	}

	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return nil, "", err
	}

	var reference name.Reference = repository.Tag(ref)
	if strings.Contains(ref, "@") || strings.HasPrefix(ref, "sha256:") {
		reference = repository.Digest(ref)
	}

	desc, err := remote.Get(reference, remoteOpts...)
	if err != nil {
		e := &transport.Error{}
		if errors.As(err, &e) {
			log.Error(e.Error())
			if e.StatusCode == http.StatusNotFound {
				return nil, "", ErrPackageNotFound
			}
		}
		return nil, "", err
	}

	return desc.Manifest, string(desc.MediaType), nil
}

func (c *DefaultClient) ListTags(ctx context.Context, log log.Logger, config *ClientConfig, path string) ([]string, error) {
	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	repository, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return nil, err
	}

	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return nil, err
	}

	tags, err := remote.List(repository, remoteOpts...)
	if err != nil {
		e := &transport.Error{}
		if errors.As(err, &e) {
			log.Error(e.Error())
			if e.StatusCode == http.StatusNotFound {
				return nil, ErrPackageNotFound
			}
		}
		return nil, err
	}

	return tags, nil
}

func newNameOptions(scheme string) []name.Option {
	opts := []name.Option{name.StrictValidation}
	if strings.ToLower(scheme) == "http" {
		opts = append(opts, name.Insecure)
	}
	return opts
}

func newRemoteOptions(ctx context.Context, config *ClientConfig) ([]remote.Option, error) {
	httpTransport := http.DefaultTransport.(*http.Transport).Clone()

	if config.CA != "" {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to load system cert pool: %w", err)
		}

		certPool.AppendCertsFromPEM([]byte(config.CA))

		httpTransport.TLSClientConfig = &tls.Config{
			RootCAs: certPool,
		}
	}

	if strings.ToLower(config.Scheme) == "http" {
		httpTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	options := []remote.Option{
		remote.WithContext(ctx),
		remote.WithTransport(httpTransport),
	}

	if config.Auth != "" {
		options = append(options, remote.WithAuth(authn.FromConfig(authn.AuthConfig{
			Auth: config.Auth,
		})))
	}

	return options, nil
}
