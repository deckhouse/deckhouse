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
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/log"
)

type DefaultClient struct{}

func (c *DefaultClient) GetPackage(ctx context.Context, log log.Logger, config *ClientConfig, digest string, path string) (int64, io.ReadCloser, error) {

	repo := config.Repository
	if path != "" {
		repo = fmt.Sprintf("%s/%s", repo, path)
	}

	nameOpts := newNameOptions(config.Scheme)
	repository, err := name.NewRepository(repo, nameOpts...)
	if err != nil {
		return 0, nil, err
	}

	remoteOpts, err := newRemoteOptions(ctx, config)
	if err != nil {
		return 0, nil, err
	}

	image, err := remote.Image(
		repository.Digest(digest),
		remoteOpts...)
	if err != nil {
		e := &transport.Error{}
		if errors.As(err, &e) {
			log.Error(e.Error())
			if e.StatusCode == http.StatusNotFound {
				return 0, nil, ErrPackageNotFound
			}
		}
		return 0, nil, err
	}

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
