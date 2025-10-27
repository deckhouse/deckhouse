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

package nelm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	helmresourcesmanager "github.com/flant/addon-operator/pkg/helm_resources_manager"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/kube-client/manifest"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	nelmServiceTracer = "nelm-service"

	chartFile    = "Chart.yaml" // Helm chart metadata file
	templatesDir = "templates"  // Helm templates directory
)

var ErrPackageNotHelm = errors.New("package not helm")

// DependencyContainer provides access to Helm resource monitoring.
type DependencyContainer interface {
	HelmResourcesManager() helmresourcesmanager.HelmResourcesManager
}

// Service manages Helm release lifecycle via nelm client.
// It provides upgrade, deletion, and rendering operations.
type Service struct {
	namespace string // Kubernetes namespace for releases
	tmpDir    string // Temporary directory for values files

	client *nelm.Client        // nelm client for Helm operations
	dc     DependencyContainer // Access to resource monitoring

	logger *log.Logger
}

// New creates a new nelm service for managing Helm releases.
func New(namespace, tmpDir string, dc DependencyContainer, logger *log.Logger) *Service {
	return &Service{
		namespace: namespace,
		tmpDir:    tmpDir,
		client:    nelm.New(namespace, logger),
		dc:        dc,
		logger:    logger.Named(nelmServiceTracer),
	}
}

// Render renders a Helm chart with the provided values and returns the manifests.
// This is useful for validating charts or previewing what will be installed.
//
// Process:
//  1. Verify the path contains a valid Helm chart
//  2. Create temporary values file
//  3. Render chart using nelm client
//  4. Return rendered YAML manifests
//
// Returns ErrPackageNotHelm if the path doesn't contain a valid Helm chart.
func (s *Service) Render(ctx context.Context, name, path string, values addonutils.Values) (string, error) {
	_, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Render")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("path", path))

	s.logger.Debug("render nelm chart", slog.String("path", path), slog.String("name", name))

	isHelm, err := s.isHelmChart(path)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("check helm chart: %w", err)
	}

	if !isHelm {
		return "", ErrPackageNotHelm
	}

	// Create temporary values file (cleaned up after rendering)
	valuesPath, err := s.createTmpValuesFile(name, values)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("create temp values file: %w", err)
	}
	defer os.Remove(valuesPath)

	return s.client.Render(ctx, name, nelm.InstallOptions{
		Path:          path,
		ValuesPaths:   []string{valuesPath},
		ReleaseLabels: nil,
	})
}

// Delete uninstalls a Helm release by name.
// This removes all resources created by the release from the cluster.
//
// Returns error if the release doesn't exist or deletion fails.
func (s *Service) Delete(ctx context.Context, name string) error {
	_, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Delete")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))

	s.logger.Debug("delete nelm release", slog.String("name", name))

	return s.client.Delete(ctx, name)
}

// Upgrade installs or upgrades a Helm release for an application.
//
// Smart upgrade logic:
//   - Renders chart and calculates manifest checksum
//   - Checks if upgrade is needed (new install, status, checksum, missing resources)
//   - Skips upgrade if nothing changed (optimization)
//   - Starts resource monitoring after successful install/upgrade
//
// Process:
//  1. Verify package contains a Helm chart
//  2. Create temporary values file from app values
//  3. Render chart to get manifests
//  4. Calculate manifest checksum
//  5. Check if upgrade is needed
//  6. Install/upgrade release if needed
//  7. Start or verify resource monitoring
//
// Returns ErrPackageNotHelm if the package doesn't contain a valid Helm chart.
func (s *Service) Upgrade(ctx context.Context, app *apps.Application) error {
	ctx, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("name", app.GetName()))
	span.SetAttributes(attribute.String("path", app.GetPath()))

	s.logger.Debug("install nelm release", slog.String("path", app.GetPath()), slog.String("name", app.GetName()))

	isHelm, err := s.isHelmChart(app.GetPath())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("check helm chart: %w", err)
	}

	if !isHelm {
		return ErrPackageNotHelm
	}

	valuesPath, err := s.createTmpValuesFile(app.GetName(), app.GetHelmValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create values file: %w", err)
	}
	defer os.Remove(valuesPath) // Clean up temp file

	// Render chart to get manifests for checksum calculation
	renderedManifests, err := s.client.Render(ctx, app.GetName(), nelm.InstallOptions{
		Path:        app.GetPath(),
		ValuesPaths: []string{valuesPath},
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("render nelm chart: %w", err)
	}

	// Calculate checksum to detect changes in rendered manifests
	checksum := addonutils.CalculateStringsChecksum(renderedManifests)

	manifests, err := manifest.ListFromYamlDocs(renderedManifests)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Determine if upgrade is actually needed (optimization)
	shouldUpgrade, err := s.shouldRunHelmUpgrade(ctx, app.GetName(), checksum, manifests)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	lastStatus := func(releaseName string) (string, string, error) {
		return s.client.LastStatus(ctx, releaseName)
	}

	if !shouldUpgrade {
		s.logger.Debug("no need to upgrade", slog.String("name", app.GetName()))

		// No upgrade needed - ensure monitoring is active
		if !s.dc.HelmResourcesManager().HasMonitor(app.GetName()) {
			s.dc.HelmResourcesManager().StartMonitor(app.GetName(), manifests, s.namespace, lastStatus)
		}

		return nil
	}

	// Install or upgrade the release
	err = s.client.Install(ctx, app.GetName(), nelm.InstallOptions{
		Path:        app.GetPath(),
		ValuesPaths: []string{valuesPath},
		ReleaseLabels: map[string]string{
			nelm.LabelPackageChecksum: checksum,
		},
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install nelm release: %w", err)
	}

	// Start monitoring resources after successful install/upgrade
	s.dc.HelmResourcesManager().StartMonitor(app.GetName(), manifests, s.namespace, lastStatus)

	return nil
}

// shouldRunHelmUpgrade determines if a Helm upgrade is needed.
//
// Upgrade is required if ANY of these conditions are true:
//  1. Release doesn't exist (revision = "0")
//  2. Release status is not "deployed" (failed/pending/etc)
//  3. Manifest checksum changed (actual changes detected)
//  4. Resources are missing in the cluster
//
// If all conditions are false, upgrade can be safely skipped (optimization).
//
// Returns:
//   - bool: true if upgrade is needed
//   - error: if checking conditions fails
func (s *Service) shouldRunHelmUpgrade(ctx context.Context, releaseName string, checksum string, manifests []manifest.Manifest) (bool, error) {
	revision, status, err := s.client.LastStatus(ctx, releaseName)
	if err != nil {
		return false, err
	}

	// First install - always run
	if revision == "0" {
		return true, nil
	}

	// Release exists but not deployed - need to upgrade to fix
	if strings.ToLower(status) != "deployed" {
		return true, nil
	}

	// Check if manifests changed by comparing checksums
	recordedChecksum, err := s.client.GetChecksum(ctx, releaseName)
	if err != nil {
		return false, err
	}

	// Checksum changed - manifests are different, need upgrade
	if recordedChecksum != checksum {
		return true, nil
	}

	// Check if any resources are missing from the cluster
	absent, err := s.dc.HelmResourcesManager().GetAbsentResources(manifests, s.namespace)
	if err != nil {
		return false, err
	}

	// Resources missing - need upgrade to recreate them
	if len(absent) > 0 {
		return true, nil
	}

	// All conditions passed - upgrade not needed
	return false, nil
}

// createTmpValuesFile creates a temporary YAML file with the provided values.
// The file is created in tmpDir with a unique name to avoid conflicts.
//
// Caller is responsible for cleaning up the file with os.Remove().
//
// Returns:
//   - string: path to the created temporary file
//   - error: if value serialization or file creation fails
func (s *Service) createTmpValuesFile(name string, values addonutils.Values) (string, error) {
	// Convert values to YAML
	data, err := values.YamlBytes()
	if err != nil {
		return "", err
	}

	// Generate unique filename to avoid conflicts
	tmpName := fmt.Sprintf("%s.package-values.yaml-%s", name, uuid.New().String())
	path := filepath.Join(s.tmpDir, tmpName)

	if err = addonutils.DumpData(path, data); err != nil {
		return "", err
	}

	return path, nil
}

// isHelmChart checks if a directory contains a valid Helm chart.
//
// A directory is considered a Helm chart if it contains either:
//  1. Chart.yaml file (standard Helm chart)
//  2. templates/ directory (minimal chart without metadata)
//
// Returns:
//   - bool: true if the path contains a Helm chart
//   - error: if filesystem check fails (not including "not exists")
func (s *Service) isHelmChart(path string) (bool, error) {
	// Check for Chart.yaml (standard Helm chart)
	_, err := os.Stat(filepath.Join(path, chartFile))
	if err == nil {
		return true, nil
	}

	// Return error if it's not "not exists" (e.g., permission denied)
	if !os.IsNotExist(err) {
		return false, err
	}

	// Check for templates/ directory (minimal chart)
	if _, err = os.Stat(filepath.Join(path, templatesDir)); err == nil {
		return true, nil
	}

	s.logger.Warn("no helm chart found in path", slog.String("path", path))

	return false, nil
}
