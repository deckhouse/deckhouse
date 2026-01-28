/*
Copyright 2025 Flant JSC

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

package installer

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
)

// Installer manages package lifecycle using a pluggable backend (erofs or symlink).
// It delegates all operations to the backend selected at construction time.
type Installer struct {
	backend Backend
}

// Backend defines the interface for package installation backends (erofs, symlink).
type Backend interface {
	Download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error
	Install(ctx context.Context, downloaded, deployed, name, version string) error
	Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error
}

// NewWithBackend creates an Installer with a custom backend.
func NewWithBackend(backend Backend) *Installer {
	return &Installer{backend: backend}
}

// Download fetches a package from the registry to <downloaded>/<version>.
func (i *Installer) Download(ctx context.Context, repo registry.Remote, downloaded, name, version string) error {
	return i.backend.Download(ctx, repo, downloaded, name, version)
}

// Install makes the downloaded package available at the deployed path.
// For symlink backend: creates symlink. For erofs backend: mounts with dm-verity.
func (i *Installer) Install(ctx context.Context, downloaded, deployed, name, version string) error {
	return i.backend.Install(ctx, downloaded, deployed, name, version)
}

// Uninstall removes the package from the deployed path.
// If keep=false, also deletes downloaded files.
func (i *Installer) Uninstall(ctx context.Context, downloaded, deployed, name string, keep bool) error {
	return i.backend.Uninstall(ctx, downloaded, deployed, name, keep)
}
