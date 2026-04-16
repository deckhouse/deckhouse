// Copyright 2025 Flant JSC
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

package client

import (
	"context"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"go.opentelemetry.io/otel"

	"github.com/deckhouse/deckhouse/pkg/registry"
	registryClient "github.com/deckhouse/deckhouse/pkg/registry/client"
)

const (
	tracerName = "registry-client"
)

// Client is a wrapper around the underlying registry client, adding tracing support.
type Client struct {
	wrapped registry.Client
}

// New creates a new wrapped internal registry client with options.
// It mirrors the public pkg registry client builder style.
func New(host string, opts ...registryClient.Option) registry.Client {
	wrapped := registryClient.New(host, opts...)
	return &Client{wrapped: wrapped}
}

// NewClientFromWrapped retains old behavior for explicit wrapping from an existing low-level client.
func NewClientFromWrapped(wrapped *registryClient.Client) registry.Client {
	return &Client{wrapped: wrapped}
}

// NewFromRegistryClient wraps any registry.Client as an internal registry Interface.
// This is useful for testing with fake or mock implementations of registry.Client.
func NewFromRegistryClient(c registry.Client) registry.Client {
	return &Client{wrapped: c}
}

func (c *Client) WithSegment(segments ...string) registry.Client {
	return &Client{wrapped: c.wrapped.WithSegment(segments...)}
}

func (c *Client) GetRegistry() string {
	return c.wrapped.GetRegistry()
}

func (c *Client) GetDigest(ctx context.Context, ref string) (*v1.Hash, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetDigest")
	defer span.End()

	return c.wrapped.GetDigest(ctx, ref)
}

func (c *Client) GetManifest(ctx context.Context, ref string) (registry.ManifestResult, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetManifest")
	defer span.End()

	return c.wrapped.GetManifest(ctx, ref)
}

func (c *Client) GetImageConfig(ctx context.Context, ref string) (*v1.ConfigFile, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetImageConfig")
	defer span.End()

	return c.wrapped.GetImageConfig(ctx, ref)
}

func (c *Client) CheckImageExists(ctx context.Context, ref string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CheckImageExists")
	defer span.End()

	return c.wrapped.CheckImageExists(ctx, ref)
}

func (c *Client) GetImage(ctx context.Context, ref string, opts ...registry.ImageGetOption) (registry.Image, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetImage")
	defer span.End()

	return c.wrapped.GetImage(ctx, ref, opts...)
}

func (c *Client) PushImage(ctx context.Context, ref string, img v1.Image, opts ...registry.ImagePushOption) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PushImage")
	defer span.End()

	return c.wrapped.PushImage(ctx, ref, img, opts...)
}

func (c *Client) ListTags(ctx context.Context, opts ...registry.ListTagsOption) ([]string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ListTags")
	defer span.End()

	return c.wrapped.ListTags(ctx, opts...)
}

func (c *Client) ListRepositories(ctx context.Context, opts ...registry.ListRepositoriesOption) ([]string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ListRepositories")
	defer span.End()

	return c.wrapped.ListRepositories(ctx, opts...)
}

func (c *Client) PushIndex(ctx context.Context, tag string, idx v1.ImageIndex, opts ...registry.ImagePushOption) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PushIndex")
	defer span.End()

	return c.wrapped.PushIndex(ctx, tag, idx, opts...)
}

func (c *Client) DeleteTag(ctx context.Context, tag string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "DeleteTag")
	defer span.End()

	return c.wrapped.DeleteTag(ctx, tag)
}

func (c *Client) DeleteByDigest(ctx context.Context, digest v1.Hash) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "DeleteByDigest")
	defer span.End()

	return c.wrapped.DeleteByDigest(ctx, digest)
}

func (c *Client) TagImage(ctx context.Context, sourceTag, destTag string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "TagImage")
	defer span.End()

	return c.wrapped.TagImage(ctx, sourceTag, destTag)
}

func (c *Client) CopyImage(ctx context.Context, srcTag string, dest registry.Client, destTag string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CopyImage")
	defer span.End()

	return c.wrapped.CopyImage(ctx, srcTag, dest, destTag)
}
