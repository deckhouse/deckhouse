/*
Copyright The ORAS Authors.
Copyright 2026 Flant JSC

Modifications made by Flant JSC as part of the Deckhouse project.

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

package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/errs"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
)

var (
	_ store.ContentStore = (*ContentStore)(nil)
)

// ContentStore implements [store.ContentStore] over a [ContentClient].
type ContentStore struct {
	client *ContentClient
}

// NewContentStore wraps layout for use as [store.ContentStore].
func NewContentStore(client *ContentClient) *ContentStore {
	return &ContentStore{client: client}
}

// Fetch implements [store.ContentStore].
func (s *ContentStore) Fetch(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	f, err := s.client.BlobOpen(dgst)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Exists implements [store.ContentStore]. After a successful [ContentClient.BlobInfo], if ctx is
// done, the returned error is ctx.Err() and exist/size still reflect the stat result.
func (s *ContentStore) Exists(ctx context.Context, dgst digest.Digest) (bool, int64, error) {
	exist, info, err := s.client.BlobInfo(dgst)
	if err == nil {
		err = ctx.Err()
	}

	var size int64
	if info != nil {
		size = info.Size()
	}
	return exist, size, err
}

// ContentClient is an OCI image layout as an [io/fs.FS]: blobs under blobs/<algo>/<hex>,
// plus oci-layout and index.json at the tree root.
type ContentClient struct {
	fsys fs.FS
}

// NewContentClient wraps fsys as an OCI layout directory.
func NewContentClient(fsys fs.FS) *ContentClient {
	return &ContentClient{fsys: fsys}
}

// BlobInfo reports whether the blob file exists for dgst and returns its size.
// A missing file yields (false, nil, nil); BlobOpen turns missing files into [errs.ErrBlobNotFound].
func (lf *ContentClient) BlobInfo(dgst digest.Digest) (bool, fs.FileInfo, error) {
	p, err := blobPath(dgst)
	if err != nil {
		return false, nil, err
	}

	info, err := fs.Stat(lf.fsys, p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, info, nil
}

// BlobOpen opens the blob for dgst. A missing file returns an error wrapping [errs.ErrBlobNotFound].
func (lf *ContentClient) BlobOpen(dgst digest.Digest) (io.ReadCloser, error) {
	p, err := blobPath(dgst)
	if err != nil {
		return nil, err
	}

	f, err := lf.fsys.Open(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w", dgst, errs.ErrBlobNotFound)
		}
		return nil, err
	}

	return f, nil
}

// ReadLayout reads and decodes the oci-layout file.
func (lf *ContentClient) ReadLayout() (ociv1.ImageLayout, error) {
	layoutPath := ociv1.ImageLayoutFile

	f, err := lf.fsys.Open(layoutPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ociv1.ImageLayout{}, fmt.Errorf("%s: %w", layoutPath, errs.ErrBlobNotFound)
		}
		return ociv1.ImageLayout{}, err
	}
	defer f.Close()

	var layout ociv1.ImageLayout
	if err := json.NewDecoder(f).Decode(&layout); err != nil {
		return ociv1.ImageLayout{}, err
	}
	return layout, nil
}

// ReadIndex reads and decodes index.json.
func (lf *ContentClient) ReadIndex() (ociv1.Index, error) {
	indexPath := ociv1.ImageIndexFile

	f, err := lf.fsys.Open(indexPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ociv1.Index{}, fmt.Errorf("%s: %w", indexPath, errs.ErrBlobNotFound)
		}
		return ociv1.Index{}, err
	}
	defer f.Close()

	var index ociv1.Index
	if err := json.NewDecoder(f).Decode(&index); err != nil {
		return ociv1.Index{}, err
	}
	return index, nil
}

// blobPath returns the layout-relative path for a validated content digest.
func blobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("%w %s: %v", errs.ErrInvalidDigest, dgst, err)
	}

	return path.Join(
		ociv1.ImageBlobsDir,
		dgst.Algorithm().String(),
		dgst.Encoded(),
	), nil
}
