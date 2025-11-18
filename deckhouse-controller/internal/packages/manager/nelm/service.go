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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/manager/nelm/monitor"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	nelmServiceTracer = "nelm-service"

	chartFile    = "Chart.yaml" // Helm chart metadata file
	templatesDir = "templates"  // Helm templates directory
)

var ErrPackageNotHelm = errors.New("package not helm")

// Service manages Helm release lifecycle via nelm client.
// It provides upgrade, deletion, and rendering operations.
type Service struct {
	tmpDir string // Temporary directory for values files

	client         *nelm.Client // nelm client for Helm operations
	monitorManager *monitor.Manager

	logger *log.Logger
}

// NewService creates a new nelm service for managing Helm releases.
func NewService(cache runtimecache.Cache, absentCallback monitor.AbsentCallback, logger *log.Logger) *Service {
	nelmClient := nelm.New(logger, nelm.WithLabels(map[string]string{
		"heritage": "deckhouse",
	}))

	return &Service{
		tmpDir:         os.TempDir(),
		client:         nelmClient,
		monitorManager: monitor.New(cache, nelmClient, absentCallback, logger),
		logger:         logger.Named(nelmServiceTracer),
	}
}

func (s *Service) HasMonitor(name string) bool {
	return s.monitorManager.HasMonitor(name)
}

func (s *Service) RemoveMonitor(name string) {
	s.monitorManager.RemoveMonitor(name)
}

func (s *Service) PauseMonitor(name string) {
	s.monitorManager.PauseMonitor(name)
}

func (s *Service) ResumeMonitor(name string) {
	s.monitorManager.ResumeMonitor(name)
}

func (s *Service) StopMonitors() {
	s.monitorManager.Stop()
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
func (s *Service) Render(ctx context.Context, app *apps.Application) (string, error) {
	_, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Render")
	defer span.End()

	span.SetAttributes(attribute.String("name", app.GetName()))
	span.SetAttributes(attribute.String("namespace", app.GetNamespace()))
	span.SetAttributes(attribute.String("path", app.GetPath()))

	s.logger.Debug("render nelm chart",
		slog.String("path", app.GetPath()),
		slog.String("namespace", app.GetNamespace()),
		slog.String("name", app.GetName()))

	isHelm, err := s.isHelmChart(app.GetPath())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("check helm chart: %w", err)
	}

	if !isHelm {
		return "", ErrPackageNotHelm
	}

	// Create temporary values file (cleaned up after rendering)
	valuesPath, err := s.createTmpValuesFile(app.GetName(), app.GetValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("create temp values file: %w", err)
	}
	defer os.Remove(valuesPath)

	return s.client.Render(ctx, app.GetNamespace(), app.GetName(), nelm.InstallOptions{
		Path:          app.GetPath(),
		ValuesPaths:   []string{valuesPath},
		ReleaseLabels: nil,
	})
}

// Delete uninstalls a Helm release by name.
// This removes all resources created by the release from the cluster.
//
// Returns error if the release doesn't exist or deletion fails.
func (s *Service) Delete(ctx context.Context, app *apps.Application) error {
	_, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Delete")
	defer span.End()

	span.SetAttributes(attribute.String("name", app.GetName()))
	span.SetAttributes(attribute.String("namespace", app.GetNamespace()))
	span.SetAttributes(attribute.String("path", app.GetPath()))

	s.logger.Debug("delete nelm release", slog.String("name", app.GetName()), slog.String("namespace", app.GetNamespace()))

	return s.client.Delete(ctx, app.GetNamespace(), app.GetName())
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
	ctx, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Upgrade")
	defer span.End()

	span.SetAttributes(attribute.String("name", app.GetName()))
	span.SetAttributes(attribute.String("namespace", app.GetNamespace()))
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

	valuesPath, err := s.createTmpValuesFile(app.GetName(), app.GetValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("create values file: %w", err)
	}
	defer os.Remove(valuesPath) // Clean up temp file

	marshalledMeta, err := json.Marshal(app.GetMetaValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("marshal metadata values: %w", err)
	}

	// Render chart to get manifests for checksum calculation
	renderedManifests, err := s.client.Render(ctx, app.GetNamespace(), app.GetName(), nelm.InstallOptions{
		Path:        app.GetPath(),
		ValuesPaths: []string{valuesPath},
		// Format as "Meta=<json>"
		ExtraValues: fmt.Sprintf("Meta=%s", marshalledMeta),
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("render nelm chart: %w", err)
	}

	// Calculate checksum to detect changes in rendered manifests
	checksum := addonutils.CalculateStringsChecksum(renderedManifests)

	// Determine if upgrade is actually needed (optimization)
	shouldUpgrade, err := s.shouldRunHelmUpgrade(ctx, app.GetNamespace(), app.GetName(), checksum)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if !shouldUpgrade {
		s.monitorManager.AddMonitor(app.GetNamespace(), app.GetName(), renderedManifests)
		s.logger.Debug("no need to upgrade", slog.String("name", app.GetName()))

		return nil
	}

	// Install or upgrade the release
	err = s.client.Install(ctx, app.GetNamespace(), app.GetName(), nelm.InstallOptions{
		Path:        app.GetPath(),
		ValuesPaths: []string{valuesPath},
		ReleaseLabels: map[string]string{
			nelm.LabelPackageChecksum: checksum,
		},
		// Format as "Meta=<json>"
		ExtraValues: fmt.Sprintf("Meta=%s", marshalledMeta),
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install nelm release: %w", err)
	}

	s.monitorManager.AddMonitor(app.GetNamespace(), app.GetName(), renderedManifests)

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
func (s *Service) shouldRunHelmUpgrade(ctx context.Context, namespace, releaseName string, checksum string) (bool, error) {
	revision, status, err := s.client.LastStatus(ctx, namespace, releaseName)
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
	recordedChecksum, err := s.client.GetChecksum(ctx, namespace, releaseName)
	if err != nil {
		return false, err
	}

	// Checksum changed - manifests are different, need upgrade
	if recordedChecksum != checksum {
		return true, nil
	}

	if err = s.monitorManager.CheckResources(ctx, releaseName); err != nil {
		if errors.Is(err, monitor.ErrAbsentManifest) {
			return true, nil
		}

		return false, err
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
