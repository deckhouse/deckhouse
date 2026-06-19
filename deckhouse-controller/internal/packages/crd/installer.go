// Copyright 2026 Flant JSC
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

// Package crd applies CRD manifests bundled inside a package to the cluster.
// Packages ship their CRDs under a top-level crds/ directory; this installer
// resolves that directory for a given package path and delegates the actual
// apply/update to the shared deckhouse CRD installer.
package crd

import (
	"context"
	"path/filepath"

	klient "github.com/flant/kube-client/client"

	d8apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
)

// crdsDir is the conventional directory inside a package that holds CRD manifests.
const crdsDir = "crds"

// Installer applies CRDs found under <packagePath>/crds/*.yaml to the cluster.
type Installer struct {
	client *klient.Client
}

// NewInstaller creates a CRD installer backed by the given Kubernetes client.
func NewInstaller(client *klient.Client) *Installer {
	return &Installer{client: client}
}

// EnsureCRDs installs or updates CRDs bundled in the package at packagePath.
// It looks up <packagePath>/crds/*.yaml; files prefixed with "doc-" are skipped
// by the underlying installer. A package without a crds/ directory is a no-op.
func (i *Installer) EnsureCRDs(ctx context.Context, packagePath string) error {
	_, err := i.EnsureCRDsReturnGVKs(ctx, packagePath)
	return err
}

// EnsureCRDsReturnGVKs installs or updates the CRDs bundled in the package at
// packagePath and returns the GroupVersionKinds of the CRDs that were applied.
// The GVK list lets callers report the freshly available API versions back to
// addon-operator (global.discovery.apiVersions). Behavior is otherwise
// identical to EnsureCRDs.
func (i *Installer) EnsureCRDsReturnGVKs(ctx context.Context, packagePath string) ([]string, error) {
	glob := filepath.Join(packagePath, crdsDir, "*.yaml")

	return d8apis.EnsureCRDsReturnGVKs(ctx, i.client, glob)
}
