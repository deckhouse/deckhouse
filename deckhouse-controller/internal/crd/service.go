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

// Package crd installs CustomResourceDefinitions bundled with Deckhouse
// packages (modules) and reports the GroupVersionKinds of Deckhouse-managed
// CRDs present in the cluster.
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
	// tracer is the OpenTelemetry tracer name for CRD installation spans.
	tracer = "crd-service"

	// crdFilters lists comma-separated filename prefixes to skip when scanning a
	// package's crds directory (documentation and partial templates).
	crdFilters = "doc-,_"

	// heritageLabelKey/heritageLabelValue mark every CRD this service applies, so
	// GetGVKs can list back exactly the CRDs Deckhouse owns.
	heritageLabelKey   = "heritage"
	heritageLabelValue = "deckhouse"

	// moduleLabelKey records the owning module name on every CRD this service
	// applies, so GetGVKs can scope results to the set of enabled modules.
	moduleLabelKey = "module"
)

// Service applies CRDs found in package paths and reports the GroupVersionKinds
// of Deckhouse-managed CRDs. It records the GVKs each module applies at install
// time, so GetManagedGVKs is served from memory without listing the cluster.
// It is safe for concurrent use.
type Service struct {
	client *client.Client
	logger *log.Logger

	// mu guards gvks.
	mu sync.RWMutex
	// gvks maps a module name to the "group/version/kind" strings of the CRDs
	// applied for it by the most recent Install. Recorded at install time
	// (crdinstaller reports them) so GetManagedGVKs needs no cluster List.
	gvks map[string][]string
}

// NewService returns a Service that applies CRDs via the given Kubernetes client.
func NewService(client *client.Client, logger *log.Logger) *Service {
	return &Service{
		client: client,
		logger: logger.Named(tracer),
		gvks:   make(map[string][]string),
	}
}

// Install scans the package's crds directory and applies every CRD manifest
// found, labelling them as Deckhouse-managed so GetGVKs can list them back.
// It is a no-op when the package ships no CRDs.
func (s *Service) Install(ctx context.Context, name, path string) error {
	ctx, span := otel.Tracer(tracer).Start(ctx, "Install")
	defer span.End()

	crds, err := getCRDsFromPath(path, crdFilters)
	if err != nil {
		return fmt.Errorf("scan crds dir: %w", err)
	}

	if len(crds) == 0 {
		// Record an empty set so a module that drops all its CRDs stops
		// contributing GVKs to capabilities on re-install.
		s.setGVKs(name, nil)
		return nil
	}

	s.logger.Debug("ensure crds", "name", name, "path", path, "crds", len(crds))

	labels := map[string]string{
		heritageLabelKey: heritageLabelValue,
		moduleLabelKey:   name,
	}

	installer := crdinstaller.NewCRDsInstaller(s.client.Dynamic(), crds, crdinstaller.WithExtraLabels(labels))

	if err := installer.Run(ctx); err != nil {
		return fmt.Errorf("failed to install crds: %w", err)
	}

	// crdinstaller records a "group/version/kind" entry for every CRD it
	// applied (created or updated), so after a successful Run this is the
	// complete GVK set for the module. Cache it for GetManagedGVKs.
	s.setGVKs(name, installer.GetAppliedGVKs())

	return nil
}

// setGVKs records the GVKs applied for a module, replacing any previous set.
func (s *Service) setGVKs(name string, gvks []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gvks[name] = gvks
}

// getCRDsFromPath returns the paths of every .yaml file under path/crds whose
// base name is not excluded by crdsFilters. A missing crds directory is not an
// error and yields an empty slice; any other walk failure is returned so the
// caller can distinguish "no CRDs" from a genuine I/O error.
func getCRDsFromPath(path string, crdsFilters string) ([]string, error) {
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
		// A package without a crds directory is the common, expected case.
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("walk crds dir: %w", err)
	}

	return crdFilesPaths, nil
}

// matchPrefix reports whether path's base name starts with any of the
// comma-separated prefixes in crdsFilters.
func matchPrefix(path string, crdsFilters string) bool {
	for filter := range strings.SplitSeq(crdsFilters, ",") {
		if strings.HasPrefix(filepath.Base(path), strings.TrimSpace(filter)) {
			return true
		}
	}

	return false
}

// GetManagedGVKs returns the GroupVersionKinds of the CRDs applied for the given
// enabled modules, as "group/version/kind" strings. The set is built from what
// each module's Install recorded, so a module whose CRDs have not been ensured
// (or that is not enabled) contributes nothing. Results are deduplicated. Returns
// nil when no modules are enabled.
func (s *Service) GetManagedGVKs(enabledModules []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	gvks := make([]string, 0)
	for _, name := range enabledModules {
		for _, gvk := range s.gvks[name] {
			if _, ok := seen[gvk]; ok {
				continue
			}

			seen[gvk] = struct{}{}
			gvks = append(gvks, gvk)
		}
	}

	return gvks
}
