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

package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"syncer/pkg/config"
	"syncer/utils/retry"
)

type Syncer struct {
	log *slog.Logger

	src, dst  name.Registry
	srcPuller *remote.Puller
	dstPuller *remote.Puller
	pusher    *remote.Pusher
}

func New(logger *slog.Logger, cfg config.Config) (*Syncer, error) {
	srcRegistry, srcOpts, err := registryOptions(cfg.Src)
	if err != nil {
		return nil, fmt.Errorf("src registry: %w", err)
	}

	dstRegistry, dstOpts, err := registryOptions(cfg.Dest)
	if err != nil {
		return nil, fmt.Errorf("dst registry: %w", err)
	}

	srcPuller, err := remote.NewPuller(srcOpts...)
	if err != nil {
		return nil, fmt.Errorf("src puller: %w", err)
	}

	dstPuller, err := remote.NewPuller(dstOpts...)
	if err != nil {
		return nil, fmt.Errorf("dst puller: %w", err)
	}

	pusher, err := remote.NewPusher(dstOpts...)
	if err != nil {
		return nil, fmt.Errorf("dst pusher: %w", err)
	}

	return &Syncer{
		log:       logger,
		src:       srcRegistry,
		dst:       dstRegistry,
		srcPuller: srcPuller,
		dstPuller: dstPuller,
		pusher:    pusher,
	}, nil
}

func (rs *Syncer) Run(ctx context.Context) error {
	startTime := time.Now()
	rs.log.Debug(
		"Sync start",
		"start_time", startTime,
		"src", rs.src.String(),
		"dst", rs.dst.String(),
	)
	defer func() {
		rs.log.Debug(
			"Sync done",
			"end_time", time.Now(),
			"duration", time.Since(startTime),
		)
	}()

	var tags []name.Tag
	if err := retry.Default().
		WithBreak(func(lastErr error) bool {
			return errors.Is(lastErr, context.Canceled)
		}).
		WithBefore(func(interval time.Duration, attempts, attempt uint, _ error) {
			rs.log.Debug(fmt.Sprintf("attempt [%d / %d] failed, next retry in %v", attempt, attempts, interval))
		}).
		Do(ctx, func() error {
			var err error
			tags, err = rs.discoverTags(ctx)
			return err
		}); err != nil {
		return fmt.Errorf("discover tags: %w", err)
	}

	total := len(tags)

	rs.log.Info(
		fmt.Sprintf("Discovered %d tags", total),
	)

	for i, tag := range tags {
		rs.log.Info(fmt.Sprintf("[%d / %d] Syncing %s", i+1, total, tag.String()))

		if err := retry.Default().
			WithBreak(func(lastErr error) bool {
				return errors.Is(lastErr, context.Canceled)
			}).
			WithBefore(func(interval time.Duration, attempts, attempt uint, _ error) {
				rs.log.Debug(fmt.Sprintf("attempt [%d / %d] failed, next retry in %v", attempt, attempts, interval))
			}).
			Do(ctx, func() error {
				return rs.syncTag(ctx, tag)
			}); err != nil {
			return fmt.Errorf("process tag: %w", err)
		}
	}
	return nil
}

func (rs *Syncer) discoverTags(ctx context.Context) ([]name.Tag, error) {
	catalogger, err := rs.srcPuller.Catalogger(ctx, rs.src)
	if err != nil {
		return nil, fmt.Errorf("create src catalogger: %w", err)
	}

	var tags []name.Tag

	for catalogger.HasNext() {
		repos, err := catalogger.Next(ctx)
		if err != nil {
			return nil, fmt.Errorf("get next src repo: %w", err)
		}

		for _, repo := range repos.Repos {
			repoName := rs.src.Repo(repo)

			lister, err := rs.srcPuller.Lister(ctx, repoName)
			if err != nil {
				return nil, fmt.Errorf("create src repo %q lister: %w", repoName.String(), err)
			}

			for lister.HasNext() {
				page, err := lister.Next(ctx)
				if err != nil {
					return nil, fmt.Errorf("get next src repo %q tags: %w", repoName.String(), err)
				}

				for _, tag := range page.Tags {
					tags = append(tags, repoName.Tag(tag))
				}
			}
		}
	}

	return tags, nil
}

func (rs *Syncer) syncTag(ctx context.Context, src name.Tag) error {
	dst := rs.dst.
		Repo(src.RepositoryStr()).
		Tag(src.TagStr())

	srcManifest, copyNeeded, err := isTagCopyNeeded(ctx, rs.srcPuller, rs.dstPuller, src, dst)
	if err != nil {
		return err
	}

	if !copyNeeded {
		rs.log.Debug("Tag already exists")
		return nil
	}

	if err = rs.pusher.Push(ctx, dst, srcManifest); err != nil {
		return fmt.Errorf("copy from %q, to %q: %w", src.String(), dst.String(), err)
	}
	return nil
}

func registryOptions(reg config.Registry) (name.Registry, []remote.Option, error) {
	var caCers []string
	if reg.CA != "" {
		caCers = append(caCers, reg.CA)
	}

	rt, err := newHTTPRoundTripper(false, caCers...)
	if err != nil {
		return name.Registry{}, nil, fmt.Errorf("cannot create http transport: %w", err)
	}

	registry, err := name.NewRegistry(reg.Address)
	if err != nil {
		return name.Registry{}, nil, fmt.Errorf("parse registry address %q: %w", reg.Address, err)
	}

	opts := []remote.Option{remote.WithTransport(rt)}
	if reg.User != nil {
		opts = append(opts, remote.WithAuth(&authn.Basic{
			Username: reg.User.Name,
			Password: reg.User.Password,
		}))
	}
	return registry, opts, nil
}

// isTagCopyNeeded checks if an image needs copying from source to destination registry.
// Returns the source manifest descriptor, whether copy is needed, and any error.
func isTagCopyNeeded(ctx context.Context, srcPuller, dstPuller *remote.Puller, src, dst name.Tag) (*remote.Descriptor, bool, error) {
	srcManifest, err := srcPuller.Get(ctx, src)
	if err != nil {
		return nil, false, fmt.Errorf("get src manifest: %w", err)
	}

	dstManifest, err := dstPuller.Get(ctx, dst)
	if err != nil {
		var tErr *transport.Error
		if errors.As(err, &tErr) &&
			(tErr.StatusCode == http.StatusNotFound || tErr.StatusCode == http.StatusForbidden) {
			// Some registries create repository on first push, so getting the manifest will fail.
			// If we see 404 or 403, assume we failed because the repository hasn't been created yet.
			return srcManifest, true, nil
		}
		return srcManifest, false, fmt.Errorf("get dst manifest: %w", err)
	}

	return srcManifest, dstManifest.Digest != srcManifest.Digest, nil
}
