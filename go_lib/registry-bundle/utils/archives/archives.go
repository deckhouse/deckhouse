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

package archives

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"

	"github.com/mholt/archives"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/chunk"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/fswrap"
)

type ReaderAtSeekerCloser interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

type FSCloser interface {
	fs.FS
	Close() error
}

type FSCloserImpl struct {
	fs.FS
	close func() error
}

func (f *FSCloserImpl) Close() error {
	return f.close()
}

type Info struct {
	BaseName string
	Chunked  bool
}

func (a Info) String() string {
	if a.Chunked {
		return fmt.Sprintf("%s.XXXX.chunk", a.BaseName)
	}
	return a.BaseName
}

func List(entries []os.DirEntry) []Info {
	ret := make([]Info, 0, len(entries))

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}

		baseName := ent.Name()
		chunked := false

		if chunk.IsChunkFile(baseName) {
			if !chunk.IsFirstChunkFile(baseName) {
				continue
			}
			baseName = chunk.BaseName(baseName)
			chunked = true
		}

		if !archives.PathIsArchive(baseName) {
			continue
		}

		ret = append(ret, Info{
			BaseName: baseName,
			Chunked:  chunked,
		})
	}
	return ret
}

func Open(dir, baseName string, chunked bool) (ReaderAtSeekerCloser, error) {
	var (
		reader ReaderAtSeekerCloser
		err    error
	)

	if chunked {
		reader, err = chunk.Open(dir, baseName)
	} else {
		reader, err = os.Open(path.Join(dir, baseName))
	}

	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	return reader, nil
}

func MountReader(reader ReaderAtSeekerCloser) (FSCloser, error) {
	ctx, ctxCancel := context.WithCancel(context.Background())

	closeFS := func() error {
		ctxCancel()
		return reader.Close()
	}

	withClose := func(err error) error {
		closeErr := closeFS()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	sysFS, err := archives.FileSystem(ctx, "", reader)
	if err != nil {
		return nil, withClose(fmt.Errorf("open filesystem: %w", err))
	}

	// https://github.com/mholt/archives/blob/71b922ebb93bac8ccc42832b17c6428ec738cdd2/fs.go#L606-L703
	if err := fs.WalkDir(sysFS, ".", func(_ string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, withClose(fmt.Errorf("indexing filesystem: %w", err))
	}

	// https://github.com/mholt/archives/blob/71b922ebb93bac8ccc42832b17c6428ec738cdd2/fs.go#L736-L742
	sysFS = fswrap.NewSubFS(sysFS)

	return &FSCloserImpl{
		FS:    sysFS,
		close: closeFS,
	}, nil
}

func Mount(dir, baseName string, chunked bool) (FSCloser, error) {
	reader, err := Open(dir, baseName, chunked)
	if err != nil {
		return nil, err
	}

	return MountReader(reader)
}
