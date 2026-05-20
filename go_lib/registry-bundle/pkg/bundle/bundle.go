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

package bundle

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/store"
	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/utils/archives"
)

type Bundle struct {
	repoStore repoStores
	archives  []archives.FSCloser
}

func New(ctx context.Context, logger log.Logger, dir string) (*Bundle, error) {
	bundle := &Bundle{
		repoStore: make(repoStores),
		archives:  make([]archives.FSCloser, 0),
	}

	withClose := func(err error) error {
		closeErr := bundle.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return err
	}

	if err := bundle.process(ctx, logger, dir); err != nil {
		return nil, withClose(err)
	}

	if err := bundle.validate(); err != nil {
		return nil, withClose(err)
	}

	if err := ctx.Err(); err != nil {
		return nil, withClose(err)
	}

	return bundle, nil
}

func (b *Bundle) Close() error {
	if b == nil {
		return nil
	}

	var err error
	for _, archive := range b.archives {
		closeErr := archive.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}

	b.repoStore = nil
	b.archives = nil
	return err
}

func (b *Bundle) process(ctx context.Context, logger log.Logger, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	archs := archives.List(entries)
	if len(archs) == 0 {
		return fmt.Errorf("no archives found in %s", dir)
	}

	for _, arch := range archs {
		logger.Infof("processing archive %s...", arch.String())
		if err := b.processArch(ctx, dir, arch.BaseName, arch.Chunked); err != nil {
			return fmt.Errorf("process %s: %w", arch.String(), err)
		}
	}
	return nil
}

func (b *Bundle) processArch(ctx context.Context, dir string, baseName string, chunked bool) error {
	sysFS, err := archives.Mount(dir, baseName, chunked)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	b.archives = append(b.archives, sysFS)

	repoStore, err := extractLegacyStore(ctx, sysFS, baseName)
	if err != nil {
		return fmt.Errorf("extract layers: %w", err)
	}

	if err := b.repoStore.merge(repoStore); err != nil {
		return fmt.Errorf("merge layers in comman store: %w", err)
	}
	return nil
}

func (b Bundle) validate() error {
	if len(b.repoStore) == 0 {
		return fmt.Errorf("bundle is empty, no layers found")
	}
	return nil
}

type repoStores map[string]store.Store

func (s repoStores) merge(src repoStores) error {
	for repo, store := range src {
		if _, exists := s[repo]; exists {
			return fmt.Errorf("duplicate repository path: %s", repo)
		}
		s[repo] = store
	}
	return nil
}
