// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	fsutils "github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

// DownloadFunc pulls imageRef and unpacks it into dir.
type DownloadFunc func(ctx context.Context, imageRef, dir string, registry RegistryConfig, showProgress bool) error

type UnpackRequest struct {
	Root   string
	Name   string
	Digest string
	// Registry is asked for only when a download is needed.
	Registry func(ctx context.Context) (*RegistryConfig, error)
}

type EnsureUnpackedOptions struct {
	// Image is the full reference to pull. Empty means <registry repo>@<digest>.
	Image string
	// Prepare runs once on the staging directory before it becomes visible as finalDir.
	Prepare func(ctx context.Context, stagingDir, finalDir string) error
	// Download replaces the registry pull. Nil means DownloadAndUnpackImage.
	Download     DownloadFunc
	ShowProgress bool
}

// EnsureUnpacked returns <root>/<name>@<digest>, downloading the image only
// when that directory does not exist yet.
func EnsureUnpacked(ctx context.Context, request UnpackRequest, options EnsureUnpackedOptions) (string, error) {
	if request.Root == "" {
		return "", errors.New("unpack image: root is required")
	}
	if request.Name == "" {
		return "", errors.New("unpack image: name is required")
	}
	if request.Digest == "" {
		return "", fmt.Errorf("unpack image %s: digest is required", request.Name)
	}
	if request.Registry == nil {
		return "", fmt.Errorf("unpack image %s@%s: registry getter is required", request.Name, request.Digest)
	}

	dir := providerdir.DigestDir(request.Root, request.Name, request.Digest)
	// Name and digest come from configuration and registries: a path separator in either leaves Root.
	if filepath.Dir(dir) != filepath.Clean(request.Root) {
		return "", fmt.Errorf("unpack image %s@%s: %s is not a directory right under %s", request.Name, request.Digest, dir, request.Root)
	}
	if err := unpack(ctx, dir, request, options); err != nil {
		return "", err
	}

	return dir, nil
}

func unpack(ctx context.Context, dir string, request UnpackRequest, options EnsureUnpackedOptions) error {
	done, err := unpacked(dir)
	if err != nil || done {
		return err
	}

	if err := os.MkdirAll(request.Root, 0o755); err != nil {
		return fmt.Errorf("create download dir %s: %w", request.Root, err)
	}

	unlock, holdsFirstLock := LockDir(ctx, dir)
	if err := ctx.Err(); err != nil {
		return errors.Join(fmt.Errorf("wait for lock of %s: %w", dir, err), unlock())
	}

	return errors.Join(unpackLocked(ctx, dir, holdsFirstLock, request, options), unlock())
}

func unpackLocked(ctx context.Context, dir string, holdsFirstLock bool, request UnpackRequest, options EnsureUnpackedOptions) error {
	// Whoever held the lock before may have unpacked this very digest.
	done, err := unpacked(dir)
	if err != nil || done {
		return err
	}

	registry, err := request.Registry(ctx)
	if err != nil {
		return fmt.Errorf("get registry for %s@%s: %w", request.Name, request.Digest, err)
	}
	if registry == nil {
		return fmt.Errorf("get registry for %s@%s: registry getter returned no registry", request.Name, request.Digest)
	}

	imageRef := options.Image
	if imageRef == "" {
		imageRef = registry.GetRegistry() + "@" + request.Digest
	}

	download := options.Download
	if download == nil {
		download = DownloadAndUnpackImage
	}

	// A second-lock holder gave up on a first-lock holder that may still be writing its staging dir.
	// ponytail: a first-lock holder after a dead one can still remove a live staging dir older than
	// StagingDirMaxAge of a second-lock or lockless process; a flock per staging dir would close that.
	if holdsFirstLock {
		removeStaleStaging(ctx, dir)
	}

	// Unpacked beside the target and renamed into place: a killed download must not leave a
	// directory that looks unpacked. The name is unique because the lock is not a
	// guarantee, and another dhctl process may be unpacking the very same digest.
	staging, err := os.MkdirTemp(request.Root, filepath.Base(dir)+".partial-")
	if err != nil {
		return fmt.Errorf("create staging dir for %s: %w", dir, err)
	}
	if err := os.Chmod(staging, 0o755); err != nil {
		return removeStaging(staging, fmt.Errorf("set staging dir mode %s: %w", staging, err))
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Downloading image %s into %s", imageRef, dir))
	if err := download(ctx, imageRef, staging, *registry, options.ShowProgress); err != nil {
		return removeStaging(staging, fmt.Errorf("download image %s: %w", imageRef, err))
	}

	if options.Prepare != nil {
		if err := options.Prepare(ctx, staging, dir); err != nil {
			return removeStaging(staging, fmt.Errorf("prepare image %s: %w", imageRef, err))
		}
	}

	if err := os.Rename(staging, dir); err != nil {
		// Another process unpacked the same digest first: its result is the same content.
		done, statErr := unpacked(dir)
		if done {
			return removeStaging(staging, nil)
		}
		return removeStaging(staging, errors.Join(fmt.Errorf("move unpacked image into place %s: %w", dir, err), statErr))
	}

	return nil
}

func unpacked(dir string) (bool, error) {
	done, err := fsutils.IsExists(dir)
	if err != nil {
		return false, fmt.Errorf("check unpacked image dir %s: %w", dir, err)
	}

	return done, nil
}

// removeStaleStaging removes the staging dirs of dir that killed downloads left. Nothing else reclaims
// them when the download dir is not the tmp dir. A failure only costs disk.
func removeStaleStaging(ctx context.Context, dir string) {
	root := filepath.Dir(dir)
	entries, err := os.ReadDir(root)
	if err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("List staging dirs of %s: %v", dir, err))
		return
	}

	prefix := filepath.Base(dir) + ".partial-"
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Stat staging dir %s: %v", entry.Name(), err))
			continue
		}
		if time.Since(info.ModTime()) < providerdir.StagingDirMaxAge {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Remove stale staging dir %s: %v", entry.Name(), err))
		}
	}
}

func removeStaging(staging string, cause error) error {
	if err := os.RemoveAll(staging); err != nil {
		return errors.Join(cause, fmt.Errorf("remove staging dir %s: %w", staging, err))
	}

	return cause
}
