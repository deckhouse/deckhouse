// Copyright 2025 Flant JSC
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

package mock

import (
	"context"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// Installer is a configurable mock. Any *Func field left nil falls back to the
// default behavior, so a zero-value &Installer{} keeps the historical stub
// semantics used across the existing tests.
type Installer struct {
	InstallFunc           func(ctx context.Context, module, version, tempModulePath string) error
	StageFunc             func(ctx context.Context, module, version, tempModulePath string) error
	StageFromRegistryFunc func(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error
	IsEmbeddedPresentFunc func(module string) bool
	UninstallFunc         func(ctx context.Context, module string) error
	DownloadFunc          func(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) (string, error)
}

func (i *Installer) Install(ctx context.Context, module, version, tempModulePath string) error {
	if i.InstallFunc != nil {
		return i.InstallFunc(ctx, module, version, tempModulePath)
	}
	return nil
}

func (i *Installer) Stage(ctx context.Context, module, version, tempModulePath string) error {
	if i.StageFunc != nil {
		return i.StageFunc(ctx, module, version, tempModulePath)
	}
	return nil
}

func (i *Installer) StageFromRegistry(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) error {
	if i.StageFromRegistryFunc != nil {
		return i.StageFromRegistryFunc(ctx, source, module, version)
	}
	return nil
}

func (i *Installer) IsEmbeddedPresent(module string) bool {
	if i.IsEmbeddedPresentFunc != nil {
		return i.IsEmbeddedPresentFunc(module)
	}
	return false
}

func (i *Installer) Uninstall(ctx context.Context, module string) error {
	if i.UninstallFunc != nil {
		return i.UninstallFunc(ctx, module)
	}
	return nil
}

func (i *Installer) Download(ctx context.Context, source *v1alpha1.ModuleSource, module, version string) (string, error) {
	if i.DownloadFunc != nil {
		return i.DownloadFunc(ctx, source, module, version)
	}
	return "testdata/validation/module", nil
}
