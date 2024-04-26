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
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type DefaultClient struct {
}

func (c *DefaultClient) GetPackage(ctx context.Context, config *ClientConfig, digest string) (int64, io.ReadCloser, error) {
	repository, err := name.NewRepository(config.Repository)
	if err != nil {
		return 0, nil, err
	}

	httpTransport := http.DefaultTransport.(*http.Transport).Clone()

	if config.CA != "" {
		certPool := x509.NewCertPool()
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

	image, err := remote.Image(
		repository.Digest(digest),
		options...)
	if err != nil {
		e := &transport.Error{}

		if errors.As(err, &e) && e.StatusCode == http.StatusNotFound {
			return 0, nil, ErrPackageNotFound
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
