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
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
)

type DefaultClient struct{}

func (c *DefaultClient) GetPackage(ctx context.Context, log log.Logger, config *ClientConfig, digest, path string) (int64, io.ReadCloser, error) {
	repo, err := buildRepository(config, path)
	if err != nil {
		return 0, nil, err
	}

	image, err := fetchImage(ctx, log, config, repo, digest)
	if err != nil {
		return 0, nil, err
	}

	return extractLastLayer(image)
}

func (c *DefaultClient) GetImage(ctx context.Context, log log.Logger, config *ClientConfig, digest, path string) (int64, io.ReadCloser, error) {
	repo, err := buildRepository(config, path)
	if err != nil {
		return 0, nil, err
	}

	image, err := fetchImage(ctx, log, config, repo, digest)
	if err != nil {
		return 0, nil, err
	}

	return createTarball(log, repo, image, digest)
}

func buildRepository(config *ClientConfig, path string) (name.Repository, error) {
	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	return name.NewRepository(repo, nameOpts...)
}

func fetchImage(ctx context.Context, log log.Logger, config *ClientConfig, repo name.Repository, digest string) (v1.Image, error) {
	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return nil, err
	}

	image, err := remote.Image(repo.Digest(digest), remoteOpts...)
	if err != nil {
		handleTransportError(log, err)
		return nil, err
	}

	return image, nil
}

func handleTransportError(log log.Logger, err error) {
	if e, ok := err.(*transport.Error); ok {
		log.Error(e.Error())
		if e.StatusCode == http.StatusNotFound {
			return
		}
	}
}

func extractLastLayer(image v1.Image) (int64, io.ReadCloser, error) {
	layers, err := image.Layers()
	if err != nil {
		return 0, nil, err
	}

	size, err := layers[len(layers)-1].Size()
	if err != nil {
		return 0, nil, err
	}

	reader, err := layers[len(layers)-1].Compressed()
	if err != nil {
		return 0, nil, err
	}

	return size, reader, nil
}

func createTarball(log log.Logger, repo name.Repository, image v1.Image, digest string) (int64, io.ReadCloser, error) {
	tag := repo.Tag(digest)
	log.Infof("Getting tar for %s\n", tag)
	refToImage := map[name.Reference]v1.Image{tag: image}
	reader, writer := io.Pipe()

	size, err := tarball.CalculateSize(refToImage)
	if err != nil {
		return 0, nil, err
	}

	log.Infof("Tarball size: %d\n", size)

	go func() {
		defer writer.Close()
		if err := tarball.Write(tag, image, writer); err != nil {
			log.Error("Failed to write tarball", err.Error())
			return
		}
	}()

	return size, reader, nil
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
