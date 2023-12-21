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
	"fmt"
	registryscaner "registry-modules-watcher/internal/backends/pkg/registry-scaner"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type registryOptions struct {
	withoutAuth bool
	dockerCfg   string
}

type Option func(options *registryOptions)

type client struct {
	registryURL string
	authConfig  authn.AuthConfig
	options     *registryOptions
}

// NewClient creates container registry client using `repo` as prefix for tags passed to methods. If insecure flag is set to true, then no cert validation is performed.
// Repo example: "cr.example.com/ns/app"
func NewClient(repo string, options ...Option) (registryscaner.Client, error) {
	opts := &registryOptions{}

	for _, opt := range options {
		opt(opts)
	}

	client := &client{
		registryURL: repo,
		options:     opts,
	}

	if !opts.withoutAuth {
		authConfig, err := readAuthConfig(repo, opts.dockerCfg)
		if err != nil {
			return nil, err
		}
		client.authConfig = authConfig
	}

	return client, nil
}

func (c *client) Name() string {
	return c.registryURL
}

func (c *client) ReleaseImage(moduleName, tag string) (v1.Image, error) {
	imageURL := c.registryURL + "/" + moduleName + "/release" + ":" + tag

	return c.image(imageURL)
}

func (c *client) Image(moduleName, tag string) (v1.Image, error) {
	imageURL := c.registryURL + "/" + moduleName + ":" + tag

	return c.image(imageURL)
}

func (c *client) image(imageURL string) (v1.Image, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(imageURL, nameOpts...) // parse options available: weak validation, etc.
	if err != nil {
		return nil, err
	}

	imageOptions := make([]remote.Option, 0)
	if !c.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(c.authConfig)))
	}

	return remote.Image(
		ref,
		imageOptions...,
	)
}

func (c *client) Modules() ([]string, error) {
	return c.list(c.registryURL)
}

func (c *client) ListTags(moduleName string) ([]string, error) {
	listTagsUrl := c.registryURL + "/" + moduleName + "/release"

	return c.list(listTagsUrl)
}

func (c *client) list(url string) ([]string, error) {
	var nameOpts []name.Option

	imageOptions := make([]remote.Option, 0)
	if !c.options.withoutAuth {
		imageOptions = append(imageOptions, remote.WithAuth(authn.FromConfig(c.authConfig)))
	}

	repo, err := name.NewRepository(url, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo %q: %w", c.registryURL, err)
	}

	return remote.List(repo, imageOptions...)
}

// WithDisabledAuth don't use authConfig
func WithDisabledAuth() Option {
	return func(options *registryOptions) {
		options.withoutAuth = true
	}
}

// WithAuth use docker config base64 as authConfig
func WithAuth(dockerCfg string) Option {
	return func(options *registryOptions) {
		options.dockerCfg = dockerCfg
	}
}
