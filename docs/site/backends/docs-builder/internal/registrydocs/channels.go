// Copyright 2026 Flant JSC
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

package registrydocs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// ChannelVersion reads the version a release channel tag points to, trying
// the sources of the module in order and the channels from the most stable
// one. A channel pointing at something that is not a version, or at a
// release image without version.json, is skipped; with no usable channel the
// error is ondemand.ErrNoChannel, ondemand.ErrChannelsGone right after a
// lookup that found one. The version comes from version.json of the release
// image; the last one of every source, module and channel is kept with its
// image digest, so an unchanged channel costs one HEAD request.
func ChannelVersion(sources ModuleSources, clients *registry.Clients) func(ctx context.Context, module string) (string, error) {
	releaseVersion := releaseVersionCache()
	// The modules whose last lookup found a channel (noChannel).
	var tagged sync.Map

	return func(ctx context.Context, module string) (string, error) {
		srcs, err := moduleSources(ctx, sources, module)
		if err != nil {
			return "", err
		}

		var errs []error
		for _, src := range srcs {
			client, err := clients.For(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			for _, channel := range ondemand.TargetChannels {
				version, err := releaseVersion(ctx, client, src.Name, module, channel)
				if errors.Is(err, registry.ErrNotFound) {
					continue
				}
				if err != nil {
					// The next source may have it.
					errs = append(errs, fmt.Errorf("source %s, channel %s: %w", src.Name, channel, err))
					break
				}
				// A development registry may point a channel at a build of a
				// branch ("mr10", "0.0.1"): not a version, try the next channel.
				if docs.ValidateModuleVersion(module, version) != nil {
					continue
				}
				tagged.Store(module, true)
				return version, nil
			}
		}

		if len(errs) > 0 {
			return "", errors.Join(errs...)
		}

		return "", noChannel(&tagged, module)
	}
}

// Channels reads the versions of all the release channels of the public
// site from the first source that has any of them. A channel pointing at
// something that is not a version is left out; with none left the error is
// ondemand.ErrNoChannel, ondemand.ErrChannelsGone right after a lookup that
// found some. A source failing on a channel is passed over for the next one,
// so the channels never mix two sources.
func Channels(sources ModuleSources, clients *registry.Clients) func(ctx context.Context, module string) (ondemand.Channels, error) {
	releaseVersion := releaseVersionCache()
	// The modules whose last lookup found channels (noChannel).
	var tagged sync.Map

	return func(ctx context.Context, module string) (ondemand.Channels, error) {
		srcs, err := moduleSources(ctx, sources, module)
		if err != nil {
			return nil, err
		}

		var errs []error
	sources:
		for _, src := range srcs {
			client, err := clients.For(src)
			if err != nil {
				errs = append(errs, fmt.Errorf("source %s: %w", src.Name, err))
				continue
			}

			found := make(ondemand.Channels)
			for _, channel := range ondemand.SiteChannels {
				version, err := releaseVersion(ctx, client, src.Name, module, channel)
				if errors.Is(err, registry.ErrNotFound) {
					continue
				}
				if err != nil {
					errs = append(errs, fmt.Errorf("source %s, channel %s: %w", src.Name, channel, err))
					continue sources
				}
				if docs.ValidateModuleVersion(module, version) == nil {
					found[channel] = version
				}
			}
			if len(found) > 0 {
				tagged.Store(module, true)
				return found, nil
			}
		}

		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}

		return nil, noChannel(&tagged, module)
	}
}

// noChannel is the error of a lookup that found no release channel of the
// module: ondemand.ErrChannelsGone if the last lookup found one (tagged),
// ondemand.ErrNoChannel otherwise. A lookup that failed is not recorded, so
// it changes nothing.
func noChannel(tagged *sync.Map, module string) error {
	if _, ok := tagged.LoadAndDelete(module); ok {
		return ondemand.ErrChannelsGone
	}

	return ondemand.ErrNoChannel
}

// releaseVersionCache reads the version a release channel of the module
// points to, from version.json of its release image, and keeps the last one
// of every source, module and channel with the image digest. An unchanged
// channel costs one HEAD request. A moved one is read by its digest, so the
// tag moving again meanwhile does not give the version of another image. A
// registry that does not answer HEAD with the digest is read by the tag, as
// before; a missing channel is an error wrapping registry.ErrNotFound.
func releaseVersionCache() func(ctx context.Context, client *registry.Client, source, module, channel string) (string, error) {
	type cached struct{ digest, version string }
	var mu sync.Mutex
	cache := make(map[string]cached)

	return func(ctx context.Context, client *registry.Client, source, module, channel string) (string, error) {
		k := source + "/" + module + "/" + channel
		digest, err := client.ReleaseDigest(ctx, module, channel)
		if errors.Is(err, registry.ErrNotFound) {
			return "", err
		}

		var img v1.Image
		switch {
		case err == nil:
			mu.Lock()
			c, ok := cache[k]
			mu.Unlock()
			if ok && c.digest == digest {
				return c.version, nil
			}
			img, err = client.ReleaseImageAt(ctx, module, digest)
			// The channel exists: the image went away after the tag moved
			// once more. Not wrapped, so it is not taken for a missing
			// channel; the next pass reads the channel again.
			if errors.Is(err, registry.ErrNotFound) {
				err = fmt.Errorf("image %s of the channel %s is gone: %v", digest, channel, err) //nolint:errorlint // see above
			}
		case errors.Is(err, registry.ErrHeadUnsupported):
			img, err = client.ReleaseImage(ctx, module, channel)
		}
		if err != nil {
			return "", err
		}

		// The digest HEAD gave is kept: for an index it is the digest of the
		// index, not of the image of the platform img is.
		if digest == "" {
			d, err := img.Digest()
			if err != nil {
				return "", fmt.Errorf("digest: %w", err)
			}
			digest = d.String()
		}

		version, err := registry.ReleaseVersion(img)
		// An image without version.json stays so; it is no version.
		if errors.Is(err, registry.ErrNoVersion) {
			version, err = "", nil
		}
		if err != nil {
			return "", err
		}

		mu.Lock()
		cache[k] = cached{digest: digest, version: version}
		mu.Unlock()

		return version, nil
	}
}
