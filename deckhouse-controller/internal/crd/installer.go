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

package crd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/flant/kube-client/client"
	"go.opentelemetry.io/otel"

	crdinstaller "github.com/deckhouse/module-sdk/pkg/crd-installer"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	tracer = "ensure-crd"

	crdsFilters = "doc-,_"
)

type packageI interface {
	GetName() string
	GetPath() string
}

type Installer struct {
	client *client.Client
	logger *log.Logger

	mu          sync.RWMutex
	appliedGVKs map[string]struct{}
}

func NewInstaller(client *client.Client, logger *log.Logger) *Installer {
	return &Installer{
		client: client,
		logger: logger.Named("crd-installer"),
	}
}

func (i *Installer) AppliedGVKs() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var gvks []string
	for gvk := range i.appliedGVKs {
		gvks = append(gvks, gvk)
	}

	return gvks
}

func (i *Installer) Install(ctx context.Context, pkg packageI) error {
	ctx, span := otel.Tracer(tracer).Start(ctx, "EnsureCRDs")
	defer span.End()

	crds := getCRDsFromPath(pkg.GetPath(), crdsFilters)
	if len(crds) == 0 {
		return nil
	}

	i.logger.Debug("ensure crds", "name", pkg.GetName(), "path", pkg.GetPath(), "crds", len(crds))

	labels := map[string]string{
		"heritage": "deckhouse",
	}

	installer := crdinstaller.NewCRDsInstaller(i.client.Dynamic(), crds, crdinstaller.WithExtraLabels(labels))
	if installer == nil {
		return nil
	}

	if err := installer.Run(ctx); err != nil {
		return fmt.Errorf("failed to install crds: %w", err)
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	for _, gvk := range installer.GetAppliedGVKs() {
		i.logger.Debug("applied gvk", "gvk", gvk)
		i.appliedGVKs[gvk] = struct{}{}
	}

	return nil
}

// getCRDsFromPath scan path/crds directory and store yaml file in slice
// if file name do not start with `_` or `doc-` prefix
func getCRDsFromPath(path string, crdsFilters string) []string {
	var crdFilesPaths []string

	err := filepath.Walk(
		filepath.Join(path, "crds"),
		func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !matchPrefix(path, crdsFilters) && filepath.Ext(path) == ".yaml" {
				crdFilesPaths = append(crdFilesPaths, path)
			}

			return nil
		})
	if err != nil {
		return nil
	}

	return crdFilesPaths
}

func matchPrefix(path string, crdsFilters string) bool {
	for filter := range strings.SplitSeq(crdsFilters, ",") {
		if strings.HasPrefix(filepath.Base(path), strings.TrimSpace(filter)) {
			return true
		}
	}

	return false
}
