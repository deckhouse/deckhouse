package proxy

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/proxy/scheduler"
	"github.com/opencontainers/go-digest"
)

type blobStore struct {
	remoteStore          distribution.BlobService
	remoteRepositoryName reference.Named
	authChallenger       authChallenger
}

var _ distribution.BlobStore = &blobStore{}

func (pbs blobStore) copyContent(ctx context.Context, dgst digest.Digest, writer io.Writer) (distribution.Descriptor, error) {
	err := pbs.authChallenger.tryEstablishChallenges(ctx)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	desc, err := pbs.remoteStore.Stat(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	if w, ok := writer.(http.ResponseWriter); ok {
		w.Header().Set("Content-Length", strconv.FormatInt(desc.Size, 10))
		w.Header().Set("Content-Type", desc.MediaType)
		w.Header().Set("Docker-Content-Digest", dgst.String())
		w.Header().Set("Etag", dgst.String())
	}

	remoteReader, err := pbs.remoteStore.Open(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	defer remoteReader.Close()

	_, err = io.CopyN(writer, remoteReader, desc.Size)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	proxyMetrics.BlobPush(uint64(desc.Size))

	return desc, nil
}

func (pbs blobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	_, err := pbs.copyContent(ctx, dgst, w)
	return err
}

func (pbs blobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	if err := pbs.authChallenger.tryEstablishChallenges(ctx); err != nil {
		return distribution.Descriptor{}, err
	}

	return pbs.remoteStore.Stat(ctx, dgst)
}

func (pbs blobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	if err := pbs.authChallenger.tryEstablishChallenges(ctx); err != nil {
		return []byte{}, err
	}

	return pbs.remoteStore.Get(ctx, dgst)
}

// Unsupported functions
func (pbs blobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	return distribution.Descriptor{}, distribution.ErrUnsupported
}

func (pbs blobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	return nil, distribution.ErrUnsupported
}

func (pbs blobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	return nil, distribution.ErrUnsupported
}

func (pbs blobStore) Mount(ctx context.Context, sourceRepo reference.Named, dgst digest.Digest) (distribution.Descriptor, error) {
	return distribution.Descriptor{}, distribution.ErrUnsupported
}

func (pbs blobStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
	return nil, distribution.ErrUnsupported
}

func (pbs blobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return distribution.ErrUnsupported
}

type cachedBlobStore struct {
	blobStore

	localStore          distribution.BlobStore
	scheduler           *scheduler.TTLExpirationScheduler
	localRepositoryName reference.Named
}

var _ distribution.BlobStore = &cachedBlobStore{}

// blobsInflight tracks currently downloading blobs
var blobsInflight = make(map[digest.Digest]struct{})

// blobsMutex protects inflight
var blobsMutex sync.Mutex

func (pbs cachedBlobStore) serveLocal(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) (bool, error) {
	localDesc, err := pbs.localStore.Stat(ctx, dgst)
	if err != nil {
		// Stat can report a zero sized file here if it's checked between creation
		// and population.  Return nil error, and continue
		return false, nil
	}

	proxyMetrics.BlobPush(uint64(localDesc.Size))
	return true, pbs.localStore.ServeBlob(ctx, w, r, dgst)
}

func (pbs cachedBlobStore) storeLocal(ctx context.Context, dgst digest.Digest) error {
	defer func() {
		blobsMutex.Lock()
		delete(blobsInflight, dgst)
		blobsMutex.Unlock()
	}()

	var (
		desc distribution.Descriptor
		err  error
		bw   distribution.BlobWriter
	)

	bw, err = pbs.localStore.Create(ctx)
	if err != nil {
		return err
	}
	defer bw.Cancel(ctx)

	desc, err = pbs.copyContent(ctx, dgst, bw)
	if err != nil {
		return err
	}

	_, err = bw.Commit(ctx, desc)
	if err != nil {
		return err
	}

	return nil
}

func (pbs cachedBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	served, err := pbs.serveLocal(ctx, w, r, dgst)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("Error serving blob from local storage: %s", err.Error())
		return err
	}

	if served {
		return nil
	}

	blobsMutex.Lock()
	_, ok := blobsInflight[dgst]
	if !ok {
		blobsInflight[dgst] = struct{}{}
	}
	blobsMutex.Unlock()

	if !ok {
		go func(dgst digest.Digest) {
			if err := pbs.storeLocal(ctx, dgst); err != nil {
				dcontext.GetLogger(ctx).Errorf("Error committing to storage: %s", err.Error())
			}

			blobRef, err := reference.WithDigest(pbs.localRepositoryName, dgst)
			if err != nil {
				dcontext.GetLogger(ctx).Errorf("Error creating reference: %s", err)
				return
			}

			if pbs.scheduler != nil {
				if err := pbs.scheduler.AddBlob(blobRef); err != nil {
					dcontext.GetLogger(ctx).Errorf("Error adding blob: %s", err)
				}
			}
		}(dgst)
	}

	return pbs.blobStore.ServeBlob(ctx, w, r, dgst)
}

func (pbs cachedBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	desc, err := pbs.localStore.Stat(ctx, dgst)
	if err == nil {
		return desc, err
	}

	if err != distribution.ErrBlobUnknown {
		return distribution.Descriptor{}, err
	}

	return pbs.blobStore.Stat(ctx, dgst)
}

func (pbs cachedBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	blob, err := pbs.localStore.Get(ctx, dgst)
	if err == nil {
		return blob, nil
	}

	blob, err = pbs.blobStore.Get(ctx, dgst)
	if err != nil {
		return []byte{}, err
	}

	_, err = pbs.localStore.Put(ctx, "", blob)
	if err != nil {
		return []byte{}, err
	}
	return blob, nil
}
