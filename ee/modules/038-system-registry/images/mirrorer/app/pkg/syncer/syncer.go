/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package syncer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"
)

type Syncer = syncer

type syncer struct {
	Src, Dst name.Registry
	Log      *slog.Logger

	SrcOptions []remote.Option
	DstOptions []remote.Option

	Parallelizm int

	srcPuller, dstPuller *remote.Puller
	pusher               *remote.Pusher

	copied, processed atomic.Int64
}

func (rs *syncer) Sync(ctx context.Context) error {
	startTime := time.Now()

	log := rs.Log.With(
		"op", "sync",
		"start_time", startTime,
		"src", rs.Src.String(),
		"dst", rs.Dst.String(),
	)

	rs.copied.Store(0)
	rs.processed.Store(0)

	log.Info("Sync start")
	defer func() {
		log.Info(
			"Sync done",
			"end_time", time.Now(),
			"duration", time.Since(startTime),
			"tags_processed", rs.processed.Load(),
			"tags_copied", rs.copied.Load(),
		)
	}()

	var err error

	if rs.srcPuller, err = remote.NewPuller(rs.SrcOptions...); err != nil {
		return fmt.Errorf("create src puller error: %w", err)
	}

	if rs.dstPuller, err = remote.NewPuller(rs.DstOptions...); err != nil {
		return fmt.Errorf("create dst puller error: %w", err)
	}

	if rs.pusher, err = remote.NewPusher(rs.DstOptions...); err != nil {
		return fmt.Errorf("create dst pusher error: %w", err)
	}

	if err = rs.discoverTags(ctx); err != nil {
		return fmt.Errorf("process tags error: %w", err)
	}

	return nil
}

func (rs *syncer) discoverTags(ctx context.Context) error {
	catalogger, err := rs.srcPuller.Catalogger(ctx, rs.Src)
	if err != nil {
		return fmt.Errorf("cannot create catalogger: %w", err)
	}

	workers := &errgroup.Group{}
	if rs.Parallelizm != 0 {
		workers.SetLimit(rs.Parallelizm)
	} else {
		workers.SetLimit(3)
	}

	for catalogger.HasNext() {
		repos, err := catalogger.Next(ctx)
		if err != nil {
			return fmt.Errorf("get next repo error: %w", err)
		}

		for _, repo := range repos.Repos {
			repoName := rs.Src.Repo(repo)

			lister, err := rs.srcPuller.Lister(ctx, repoName)
			if err != nil {
				return fmt.Errorf("cannot create repo %v lister: %w", repoName.String(), err)
			}

			for lister.HasNext() {
				tags, err := lister.Next(ctx)
				if err != nil {
					return fmt.Errorf("get next tag in repo %v error: %w", repoName.String(), err)
				}

				for _, tag := range tags.Tags {
					tagName := repoName.Tag(tag)

					workers.Go(func() error {
						rs.handleTag(ctx, tagName)
						return nil
					})
				}
			}
		}
	}

	if err = workers.Wait(); err != nil {
		return fmt.Errorf("handle tags error: %w", err)
	}

	return nil
}

func (rs *syncer) handleTag(ctx context.Context, src name.Tag) {
	rs.processed.Add(1)

	dst := rs.Dst.
		Repo(src.RepositoryStr()).
		Tag(src.TagStr())

	log := rs.Log.With(
		"op", "handleTag",
		"src", src.String(),
		"dst", dst.String(),
	)

	srcManifest, copyNedded := rs.isTagCopyNeeded(ctx, src, dst)
	if !copyNedded {
		return
	}

	copyStartTime := time.Now()
	log.Debug("Copy tag start", "start_time", copyStartTime)

	if err := rs.pusher.Push(ctx, dst, srcManifest); err != nil {
		log.Error(
			"Copy tag error",
			"src", src.String(),
			"dst", dst.String(),
			"error", err,
		)
	}

	tagsCount := rs.copied.Add(1)
	copyEndTime := time.Now()

	log.Debug(
		"Copy tag done",
		"start_time", copyStartTime,
		"end_time", copyEndTime,
		"duration", copyEndTime.Sub(copyStartTime),
		"tags_copied", tagsCount,
	)
}

func (rs *syncer) isTagCopyNeeded(ctx context.Context, src, dst name.Tag) (srcManifest *remote.Descriptor, copyNeeded bool) {
	log := rs.Log.With(
		"op", "isTagCopyNeeded",
		"src", src.String(),
		"dst", dst.String(),
	)

	srcManifest, err := rs.srcPuller.Get(ctx, src)
	if err != nil {
		log.Error("Cannot get src manifest", "error", err)
		return
	}

	dstManifest, err := rs.dstPuller.Get(ctx, dst)
	if err != nil {
		var tErr *transport.Error
		if errors.As(err, &tErr) &&
			tErr.StatusCode == http.StatusNotFound || tErr.StatusCode == http.StatusForbidden {
			// Some registries create repository on first push, so listing tags will fail.
			// If we see 404 or 403, assume we failed because the repository hasn't been created yet.

			copyNeeded = true
			return
		} else {
			log.Error("Cannot get dst manifest", "error", err)
			return
		}
	}

	if dstManifest.Digest == srcManifest.Digest {
		return
	}

	if srcManifest.MediaType.IsImage() && dstManifest.MediaType.IsImage() {
		srcCfg, err := getImageConfigFile(srcManifest)
		if err != nil {
			log.Error("Cannot get src image config file", "error", err)
			return
		}

		dstCfg, err := getImageConfigFile(dstManifest)
		if err != nil {
			log.Error("Cannot get dst image config file", "error", err)
			return
		}

		if dstCfg.Created.Time.After(srcCfg.Created.Time) {
			log.Warn(
				"Skipping image due dst created time is newer than src",
				"src_created", srcCfg.Created.Time,
				"dst_created", dstCfg.Created.Time,
			)

			return
		}
	}

	copyNeeded = true
	return
}
