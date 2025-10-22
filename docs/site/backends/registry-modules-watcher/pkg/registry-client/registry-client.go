// Copyright 2023 Flant JSC
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

package registryclient

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	registryscanner "registry-modules-watcher/internal/backends/pkg/registry-scanner"
	"registry-modules-watcher/internal/metrics"
)

type registryOptions struct {
	withoutAuth bool
	dockerCfg   string
}

type Option func(options *registryOptions)

type client struct {
	registryURL   string
	authConfig    authn.AuthConfig
	options       *registryOptions
	metricStorage *metricsstorage.MetricStorage
}

// NewClient creates container registry client using `repo` as prefix for tags passed to methods. If insecure flag is set to true, then no cert validation is performed.
// Repo example: "cr.example.com/ns/app"
func NewClient(repo string, metricStorage *metricsstorage.MetricStorage, options ...Option) (registryscanner.Client, error) {
	opts := &registryOptions{}

	for _, opt := range options {
		opt(opts)
	}

	client := &client{
		registryURL:   repo,
		metricStorage: metricStorage,
		options:       opts,
	}

	if !opts.withoutAuth {
		authConfig, err := readAuthConfig(repo, opts.dockerCfg)
		if err != nil {
			return nil, fmt.Errorf("read auth config: %w", err)
		}

		client.authConfig = authConfig
	}

	return client, nil
}

// Returns registry URL
func (c *client) Name() string {
	return c.registryURL
}

func (c *client) ReleaseImage(ctx context.Context, moduleName, tag string) (v1.Image, error) {
	imageURL := c.registryURL + "/" + moduleName + "/release" + ":" + tag

	return c.image(ctx, imageURL)
}

func (c *client) Image(ctx context.Context, moduleName, tag string) (v1.Image, error) {
	imageURL := c.registryURL + "/" + moduleName + ":" + tag

	return c.image(ctx, imageURL)
}

func (c *client) image(ctx context.Context, imageURL string) (v1.Image, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(imageURL, nameOpts...) // parse options available: name.WeakValidation, etc.
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	imageOptions := make([]remote.Option, 0)
	if !c.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(c.authConfig)))
	}

	imageOptions = append(imageOptions, metrics.RoundTripOption(c.metricStorage)) // calculace metrics
	imageOptions = append(imageOptions, remote.WithContext(ctx))

	return remote.Image(
		ref,
		imageOptions...,
	)
}

func (c *client) Modules(ctx context.Context) ([]string, error) {
	return c.list(ctx, c.registryURL)
}

func (c *client) ListTags(ctx context.Context, moduleName string) ([]string, error) {
	listTagsURL := c.registryURL + "/" + moduleName + "/release"

	return c.list(ctx, listTagsURL)
}

func (c *client) list(ctx context.Context, url string) ([]string, error) {
	var nameOpts []name.Option

	imageOptions := make([]remote.Option, 0)
	if !c.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(c.authConfig)))
	}

	imageOptions = append(imageOptions, metrics.RoundTripOption(c.metricStorage))
	imageOptions = append(imageOptions, remote.WithContext(ctx))

	repo, err := name.NewRepository(url, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", url, err)
	}

	return remote.List(repo, imageOptions...)
}

// WithDisabledAuth disables the use of authConfig
func WithDisabledAuth() Option {
	return func(options *registryOptions) {
		options.withoutAuth = true
	}
}

// WithAuth sets the docker config base64 as authConfig
func WithAuth(dockerCfg string) Option {
	return func(options *registryOptions) {
		options.dockerCfg = dockerCfg
	}
}
