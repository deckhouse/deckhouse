package proxy

import (
	"context"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/proxy/scheduler"
	"github.com/opencontainers/go-digest"
)

type manifestStore struct {
	remoteManifests      distribution.ManifestService
	remoteRepositoryName reference.Named
	authChallenger       authChallenger
}

var _ distribution.ManifestService = &manifestStore{}

func (pms manifestStore) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	if err := pms.authChallenger.tryEstablishChallenges(ctx); err != nil {
		return false, err
	}
	return pms.remoteManifests.Exists(ctx, dgst)
}

func (pms manifestStore) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	if err := pms.authChallenger.tryEstablishChallenges(ctx); err != nil {
		return nil, err
	}

	manifest, err := pms.remoteManifests.Get(ctx, dgst, options...)
	if err != nil {
		return nil, err
	}

	_, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	proxyMetrics.ManifestPush(uint64(len(payload)))
	proxyMetrics.ManifestPull(uint64(len(payload)))

	return manifest, err
}

func (pms manifestStore) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	var d digest.Digest
	return d, distribution.ErrUnsupported
}

func (pms manifestStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return distribution.ErrUnsupported
}

type cachedManifestStore struct {
	manifestStore

	localManifests      distribution.ManifestService
	localRepositoryName reference.Named
	scheduler           *scheduler.TTLExpirationScheduler
}

var _ distribution.ManifestService = &cachedManifestStore{}

func (pms cachedManifestStore) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	exists, err := pms.localManifests.Exists(ctx, dgst)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}
	return pms.manifestStore.Exists(ctx, dgst)
}

func (pms cachedManifestStore) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	// At this point `dgst` was either specified explicitly, or returned by the
	// tagstore with the most recent association.
	manifest, err := pms.localManifests.Get(ctx, dgst, options...)
	if err == nil {
		_, payload, err := manifest.Payload()
		if err != nil {
			return nil, err
		}
		proxyMetrics.ManifestPush(uint64(len(payload)))
		return manifest, nil
	}

	manifest, err = pms.manifestStore.Get(ctx, dgst, options...)
	if err != nil {
		return nil, err
	}

	_, err = pms.localManifests.Put(ctx, manifest)
	if err != nil {
		return nil, err
	}

	// Schedule the manifest blob for removal
	repoBlob, err := reference.WithDigest(pms.localRepositoryName, dgst)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("Error creating reference: %s", err)
		return nil, err
	}

	if pms.scheduler != nil {
		if err := pms.scheduler.AddManifest(repoBlob); err != nil {
			dcontext.GetLogger(ctx).Errorf("Error adding manifest: %s", err)
		}
	}
	// Ensure the manifest blob is cleaned up
	//pms.scheduler.AddBlob(blobRef, repositoryTTL)

	return manifest, err
}
