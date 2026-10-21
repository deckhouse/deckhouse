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
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/kube-client/manifest"
	"github.com/google/uuid"
	"github.com/werf/nelm/pkg/common"
	"github.com/werf/nelm/pkg/legacy/progrep"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm/drift"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcerequests"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	nelmServiceTracer = "nelm-service"

	// Operations of helm_operation_seconds: addon-operator's template and upgrade, and delete.
	operationTemplate = "template"
	operationUpgrade  = "upgrade"
	operationDelete   = "delete"

	chartFile    = "Chart.yaml" // Helm chart metadata file
	templatesDir = "templates"  // Helm templates directory
	chartsDir    = "charts"     // Helm subcharts directory
	webhooksDir  = "webhooks"   // webhook handlers read by templates via .Files.Get

	// conversionWebhookKind is the kind of the conversion webhook manifests.
	conversionWebhookKind = "ConversionWebhook"
)

const (
	// timeoutGrace keeps nelm's own deadline behind ours: both bound the same
	// apply, but only ours cancels with a cause naming what it waited for.
	timeoutGrace = time.Minute

	// maxNamedResources bounds how many resources a timeout cause names — the
	// text reaches a package condition message.
	maxNamedResources = 5
)

// MaintenanceState is the reconciliation mode requested for a package. The zero
// value (Managed) reconciles normally; it mirrors ModuleConfig maintenance.
type MaintenanceState string

const (
	// Managed reconciles the package's resources normally.
	Managed MaintenanceState = ""
	// NoResourceReconciliation stops reconciling the package's resources: the Helm
	// release is left untouched, resource monitoring is disabled, and resources are
	// labeled so they can be edited by hand.
	NoResourceReconciliation MaintenanceState = "NoResourceReconciliation"
)

var ErrPackageNotHelm = errors.New("package not helm")

// chartFileExtensions are the chart files that can render a manifest. Packaged
// subcharts (.tgz) are not inspected.
var chartFileExtensions = []string{".yaml", ".yml", ".tpl"}

// conversionWebhookKindLine matches a literal `kind: ConversionWebhook` line, so a
// mere mention of the kind (e.g. in a comment) does not select the template.
var conversionWebhookKindLine = regexp.MustCompile(`(?m)^kind:\s*` + conversionWebhookKind + `\s*$`)

// Package provides access to package data needed for Helm operations.
type Package interface {
	GetName() string
	GetPath() string
	GetRuntimeValues() string
	GetValues() addonutils.Values
	// GetMaintenance reports the package's maintenance mode; the package itself
	// decides whether its resources must be reconciled.
	GetMaintenance() MaintenanceState
}

// application interface abstracts application operations needed for the run cycle.
type application interface {
	GetInstance() string
	GetPackage() string
}

// resourceSizer is implemented by packages that carry per-workload resource
// overrides from their CR. Optional, like application above: modules do not
// implement it.
type resourceSizer interface {
	GetResourceRequests() []resourcerequests.Request
}

// Service manages Helm release lifecycle via nelm client.
// It provides upgrade, deletion, and rendering operations.
type Service struct {
	tmpDir string // Temporary directory for values files

	client         *nelm.Client // nelm client for Helm operations
	monitorManager *drift.Manager

	timeout time.Duration // bounds one release apply

	// observations receive the release progress, the endpoints and "no workloads" (observe.go).
	observations observations
	// metrics receive the helm timings addon-operator wrote; nil records none.
	metrics metricsstorage.Storage

	logger *log.Logger
}

// NewService creates a new nelm service for managing Helm releases. observations receive what the
// service observes while it renders, applies and deletes releases (observed.Store); metricStorage
// the timings of those operations.
func NewService(kubeClient *klient.Client, observations observations, metricStorage metricsstorage.Storage, logger *log.Logger) *Service {
	timeout := envconfig.PackageNelmTimeout()

	nelmClient := nelm.New(logger,
		nelm.WithResourcesLabels(map[string]string{
			"heritage": "deckhouse",
		}),
		nelm.WithReleaseAnnotations(map[string]string{
			v1alpha1.PackageAnnotationManagedBy: v1alpha1.PackageAnnotationManagedByValue,
		}),
		// nelm's deadline is a backstop behind ours: a non-zero Timeout is what makes
		// ReleaseInstall return context.Cause rather than its own unwind error.
		nelm.WithTimeout(timeout+timeoutGrace),
	)

	return &Service{
		tmpDir:         os.TempDir(),
		client:         nelmClient,
		timeout:        timeout,
		observations:   observations,
		metrics:        metricStorage,
		monitorManager: drift.New(kubeClient, nelmClient, logger),
		logger:         logger.Named(nelmServiceTracer),
	}
}

// HasMonitor checks if a release monitor exists for the given name.
func (s *Service) HasMonitor(name string) bool {
	return s.monitorManager.HasMonitor(name)
}

// RemoveMonitor stops and removes a release monitor. No-op if the monitor doesn't exist.
func (s *Service) RemoveMonitor(name string) {
	s.monitorManager.RemoveMonitor(name)
}

// PauseMonitor pauses resource readiness checks for a release monitor.
// Requires an equal number of ResumeMonitor calls to unpause.
func (s *Service) PauseMonitor(name string) {
	s.monitorManager.PauseMonitor(name)
}

// ResumeMonitor decrements the pause counter for a release monitor.
// The monitor resumes checking resources when the counter reaches zero.
func (s *Service) ResumeMonitor(name string) {
	s.monitorManager.ResumeMonitor(name)
}

// StopMonitors gracefully shuts down all release monitors.
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
func (s *Service) Render(ctx context.Context, namespace string, pkg Package) (string, error) {
	ctx, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Render")
	defer span.End()

	span.SetAttributes(attribute.String("package", pkg.GetName()))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", pkg.GetPath()))

	s.logger.Debug("render nelm chart",
		slog.String("path", pkg.GetPath()),
		slog.String("namespace", namespace),
		slog.String("name", pkg.GetName()))

	isHelm, err := s.isHelmChart(pkg.GetPath())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("check helm chart: %w", err)
	}

	if !isHelm {
		return "", ErrPackageNotHelm
	}

	// Create temporary values file (cleaned up after rendering)
	valuesPath, err := s.createTmpValuesFile(pkg.GetName(), pkg.GetValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("create temp values file: %w", err)
	}
	defer os.Remove(valuesPath)

	opts := nelm.InstallOptions{
		Path:        pkg.GetPath(),
		ValuesPaths: []string{valuesPath},
		RootValues:  pkg.GetRuntimeValues(),
		ResourcesLabels: map[string]string{
			health.LabelKey: pkg.GetName(),
		},
	}

	requests := resourceRequests(pkg)
	if len(requests) == 0 {
		return s.client.Render(ctx, namespace, pkg.GetName(), opts)
	}

	// Render what the package actually gets: the runtime dump reads this, and it
	// would be misleading if it showed the chart's own sizing rather than the
	// overlay the install applies.
	rendered, err := s.renderResized(ctx, namespace, pkg.GetName(), opts, requests)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", err
	}

	return nelm.JoinManifests(rendered)
}

// resourceRequests returns the package's per-workload resource overrides, or nil
// when the feature gate is off or the package carries none.
//
// The gate is read here, at the single point the overlay enters the release path,
// so turning it off leaves both the rendered manifests and the chart the release
// installs from exactly as they were.
func resourceRequests(pkg Package) []resourcerequests.Request {
	if !app.ResourceRequestsEnabled() {
		return nil
	}

	sizer, ok := pkg.(resourceSizer)
	if !ok {
		return nil
	}

	return sizer.GetResourceRequests()
}

// renderResized renders the chart and overlays the package's resource requests on
// the result, returning the resources with the classification nelm gave them.
func (s *Service) renderResized(ctx context.Context, namespace, name string, opts nelm.InstallOptions, requests []resourcerequests.Request) ([]nelm.RenderedResource, error) {
	rendered, err := s.client.RenderResources(ctx, namespace, name, opts)
	if err != nil {
		return nil, fmt.Errorf("render resources: %w", err)
	}

	resources := make([]*unstructured.Unstructured, 0, len(rendered))
	for _, resource := range rendered {
		resources = append(resources, resource.Unstruct)
	}

	unmatched, err := resourcerequests.Apply(resources, requests)
	if err != nil {
		return nil, fmt.Errorf("apply resource requests: %w", err)
	}

	// A chart may render a workload conditionally, so a dormant request is not an
	// error — but a typo in kind or name looks exactly the same from here, and
	// silently sizing nothing is the worse failure of the two.
	for _, request := range unmatched {
		s.logger.Warn("resource request matches no rendered workload",
			slog.String("name", name),
			slog.String("kind", request.Kind),
			slog.String("workload", request.Name))
	}

	return rendered, nil
}

// writeResizedChart materialises the overlaid resources as a chart to install
// from, and returns its path along with the cleanup the caller must defer.
//
// Standalone crds/ CRDs are left out: Install passes NoInstallStandaloneCRDs, so
// carrying them into the overlay's templates/ would start applying CRDs this
// release has never applied. Hooks are carried over, and nelm reclassifies them
// from their own annotations when it renders the overlay.
func (s *Service) writeResizedChart(name, srcChart string, rendered []nelm.RenderedResource) (string, func(), error) {
	resources := make([]*unstructured.Unstructured, 0, len(rendered))
	for _, resource := range rendered {
		if !resource.Regular && !resource.Hook {
			continue
		}

		resources = append(resources, resource.Unstruct)
	}

	dir := filepath.Join(s.tmpDir, fmt.Sprintf("%s.package-chart-%s", name, uuid.New().String()))
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			s.logger.Warn("failed to remove overlay chart", slog.String("path", dir), log.Err(err))
		}
	}

	if err := resourcerequests.WriteChart(dir, srcChart, resources); err != nil {
		cleanup()

		return "", func() {}, fmt.Errorf("write overlay chart: %w", err)
	}

	return dir, cleanup, nil
}

// Delete uninstalls a Helm release by name.
// This removes all resources created by the release from the cluster.
//
// Returns error if the release doesn't exist or deletion fails.
func (s *Service) Delete(ctx context.Context, namespace, name string) error {
	ctx, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Delete")
	defer span.End()

	span.SetAttributes(attribute.String("package", name))
	span.SetAttributes(attribute.String("namespace", namespace))

	s.logger.Debug("delete nelm release", slog.String("name", name), slog.String("namespace", namespace))

	started := time.Now()
	err := s.client.Delete(ctx, namespace, name, s.trackUninstall)
	s.observeOperation(operationDelete, namespace, name, started)

	return err
}

// Upgrade installs or upgrades a Helm release for a package.
//
// Smart upgrade logic:
//   - Renders chart and calculates manifest checksum
//   - Checks if upgrade is needed (new install, status, checksum, missing resources)
//   - Skips upgrade if nothing changed (optimization)
//   - Starts resource monitoring after successful install/upgrade
//
// Process:
//  1. Verify package contains a Helm chart
//  2. Create temporary values file from package values
//  3. Render chart to get manifests
//  4. Calculate manifest checksum
//  5. Check if upgrade is needed
//  6. Install/upgrade release if needed
//  7. Start or verify resource monitoring
//
// Maintenance (state == NoResourceReconciliation): once the release is marked as
// under maintenance the upgrade is skipped and resource monitoring is dropped, so
// config/hook changes are not reconciled and deleted resources are not restored.
// Resources are stamped with the maintenance label so the deckhouse admission
// policy stops guarding them against manual edits.
//
// Returns ErrPackageNotHelm if the package doesn't contain a valid Helm chart.
//
// applying, when not nil, is called once right before the release is installed or upgraded, and
// never when the render matches the deployed release: the caller reports the apply in flight
// (ManifestsApplied false with ApplyingManifests, as the status did when nelm sent progress).
//
// drifted is the callback of the drift monitor this call (re)starts for the release: it is called
// whenever resources of this render are absent. The caller of the last Upgrade owns the monitor,
// so a new owner of the release (a switch) takes it over with its first Upgrade.
func (s *Service) Upgrade(ctx context.Context, namespace string, pkg Package, applying, drifted func()) error {
	ctx, span := otel.Tracer(nelmServiceTracer).Start(ctx, "Upgrade")
	defer span.End()

	// The package owns its maintenance mode and decides whether it reconciles.
	state := pkg.GetMaintenance()

	span.SetAttributes(attribute.String("package", pkg.GetName()))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", pkg.GetPath()))

	s.logger.Debug("install nelm release", slog.String("path", pkg.GetPath()), slog.String("name", pkg.GetName()))

	isHelm, err := s.isHelmChart(pkg.GetPath())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newReasonError(reasonCheckTemplatesFailed, err)
	}

	if !isHelm {
		// No chart means no release and no workloads, so the health monitor never reports on the
		// package; recorded here, by the only writer of the observation.
		s.observations.SetNoWorkloads(pkg.GetName(), true)

		return ErrPackageNotHelm
	}

	defer s.observeUpgrade(namespace, pkg.GetName(), time.Now())

	// Once the release is marked as under maintenance, stop reconciling: skip the
	// upgrade and drop monitoring so deleted resources are not restored.
	if state == NoResourceReconciliation {
		marked, err := s.isReleaseUnderMaintenance(ctx, namespace, pkg.GetName())
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return newReasonError(reasonCheckReleaseFailed, err)
		}

		if marked {
			s.monitorManager.RemoveMonitor(pkg.GetName())
			s.logger.Debug("release under maintenance, skip upgrade", slog.String("name", pkg.GetName()))

			return nil
		}
	}

	valuesPath, err := s.createTmpValuesFile(pkg.GetName(), pkg.GetValues())
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newReasonError(reasonCreateValuesFileFailed, err)
	}
	defer os.Remove(valuesPath) // Clean up temp file

	// The maintenance marker lives on the resources, so toggling it changes the
	// rendered-manifest checksum and forces exactly one upgrade on enter/leave.
	resourcesLabels := map[string]string{
		health.LabelKey:              pkg.GetName(),
		v1alpha1.PackageLabelPackage: pkg.GetName(),
	}
	if state == NoResourceReconciliation {
		resourcesLabels[nelm.ReleaseLabelMaintenance] = ""
	}

	trackingOptions := common.TrackingOptions{
		NoPodLogs: true,
	}

	if app, ok := pkg.(application); ok {
		resourcesLabels[v1alpha1.PackageLabelInstance] = app.GetInstance()
		// application has separate package name
		resourcesLabels[v1alpha1.PackageLabelPackage] = app.GetPackage()
	} else {
		// options needed for modules
		trackingOptions.NoFinalTracking = true
		trackingOptions.LegacyHelmCompatibleTracking = true
	}

	s.logger.Debug("render nelm chart",
		slog.String("path", pkg.GetPath()),
		slog.String("name", pkg.GetName()),
		slog.String("namespace", namespace))

	renderOptions := nelm.InstallOptions{
		Path:            pkg.GetPath(),
		ValuesPaths:     []string{valuesPath},
		RootValues:      pkg.GetRuntimeValues(),
		ResourcesLabels: resourcesLabels,
	}

	// Render chart to get manifests for checksum calculation; resized holds the overlaid resources,
	// kept around to build the chart the release is installed from.
	renderStarted := time.Now()
	renderedManifests, resized, err := s.renderRelease(ctx, namespace, pkg, renderOptions)
	s.observeOperation(operationTemplate, namespace, pkg.GetName(), renderStarted)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newReasonError(reasonRenderFailed, err)
	}

	// Collect application endpoint URLs from the rendered manifests. Done
	// before the shouldUpgrade check so URLs are refreshed even when the
	// upgrade itself is skipped.
	s.observations.UpdateURLs(pkg.GetName(), extractEndpointURLs(renderedManifests))

	// The health monitor reports only on workloads it sees, so a release without
	// any would leave Scaled Unknown and the first install never complete.
	s.observations.SetNoWorkloads(pkg.GetName(), !hasWorkloads(renderedManifests))

	// Calculate checksum to detect changes in rendered manifests
	checksum := addonutils.CalculateStringsChecksum(renderedManifests)

	// Determine if upgrade is actually needed (optimization)
	shouldUpgrade, err := s.shouldRunHelmUpgrade(ctx, namespace, pkg.GetName(), checksum)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newReasonError(reasonCheckReleaseFailed, err)
	}

	if !shouldUpgrade {
		s.updateMonitor(state, namespace, pkg.GetName(), renderedManifests, drifted)
		s.logger.Debug("no need to upgrade", slog.String("name", pkg.GetName()))

		return nil
	}

	// chartPath is what the release is installed from: the package's own chart, or
	// an overlay chart holding the resized manifests, since nelm renders the chart
	// it is handed rather than taking manifests. Built here and not next to the
	// render because most calls stop at the check above, and the overlay is a
	// directory written to disk.
	chartPath := pkg.GetPath()

	if len(resized) > 0 {
		dir, cleanup, err := s.writeResizedChart(pkg.GetName(), pkg.GetPath(), resized)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return newReasonError(reasonRenderFailed, err)
		}
		defer cleanup()

		chartPath = dir
	}

	// The observed progress still holds the previous apply's report; it is not this one's.
	s.observations.ResetTracking(pkg.GetName())

	if applying != nil {
		applying()
	}

	// The deadline is ours, so an apply that outlives it is cancelled with a cause
	// naming what it waited for. The apply keeps
	// its own progress for that message (applyProgress).
	progress := new(applyProgress)

	ctx, stopDeadline := s.withApplyDeadline(ctx, progress)
	defer stopDeadline()

	// Install or upgrade the release
	installStarted := time.Now()
	err = s.client.Install(ctx, namespace, pkg.GetName(), nelm.InstallOptions{
		OnTrackingEvent: s.trackApply(progress),
		TrackingOptions: trackingOptions,
		Path:            chartPath,
		ValuesPaths:     []string{valuesPath},
		RootValues:      pkg.GetRuntimeValues(),
		ReleaseLabels: map[string]string{
			nelm.ReleaseLabelPackageChecksum: checksum,
			nelm.ReleaseLabelMaintenance:     strconv.FormatBool(state == NoResourceReconciliation),
			// TODO(ipaqsa): addon-operator legacy to migrate
			nelm.ReleaseLabelModuleChecksum: checksum,
		},
		ResourcesLabels: resourcesLabels,
	})
	s.observeOperation(operationUpgrade, namespace, pkg.GetName(), installStarted)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newReasonError(reasonApplyManifestsFailed, err)
	}

	s.updateMonitor(state, namespace, pkg.GetName(), renderedManifests, drifted)

	return nil
}

// isReleaseUnderMaintenance reports whether the release is deployed and already
// marked as under maintenance. A missing, failed, or pending release — or a missing
// label — means "not (successfully) marked", so the caller must (re)apply to stamp
// it. The deployed check matters because nelm writes the maintenance storage label
// at pending time: without it a failed enter-maintenance install would be frozen
// and falsely reported as applied until maintenance is turned off.
func (s *Service) isReleaseUnderMaintenance(ctx context.Context, namespace, name string) (bool, error) {
	_, releaseStatus, err := s.client.LastStatus(ctx, namespace, name)
	if err != nil {
		return false, fmt.Errorf("get release status: %w", err)
	}

	if strings.ToLower(releaseStatus) != "deployed" {
		return false, nil
	}

	value, err := s.client.GetReleaseLabel(ctx, namespace, name, nelm.ReleaseLabelMaintenance)
	if err != nil {
		if errors.Is(err, nelm.ErrReleaseNotFound) || errors.Is(err, nelm.ErrLabelNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("get maintenance label: %w", err)
	}

	// Pairs with strconv.FormatBool written on install.
	return value == "true", nil
}

// updateMonitor keeps resource monitoring on for managed packages and off for
// packages under maintenance, so their deleted resources are not restored.
func (s *Service) updateMonitor(state MaintenanceState, namespace, name, manifests string, drifted func()) {
	if state == NoResourceReconciliation {
		s.monitorManager.RemoveMonitor(name)
		return
	}

	s.monitorManager.AddMonitor(namespace, name, manifests, drifted)
}

// HasConversionWebhook reports whether the package's chart can render a
// ConversionWebhook, so callers skip the render for packages that ship none.
// It scans the chart's templates and subcharts for a literal `kind:` line, so a
// kind built dynamically (`kind: {{ $kind }}`) is not detected.
func (s *Service) HasConversionWebhook(path string) (bool, error) {
	for _, dir := range []string{templatesDir, chartsDir} {
		found, err := containsMatch(filepath.Join(path, dir), conversionWebhookKindLine)
		if err != nil {
			return false, fmt.Errorf("scan %s dir: %w", dir, err)
		}

		if found {
			return true, nil
		}
	}

	return false, nil
}

// containsMatch reports whether any chart file under root matches re.
// A missing root is not an error: the chart simply has no such directory.
func containsMatch(root string, re *regexp.Regexp) (bool, error) {
	found := false

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !slices.Contains(chartFileExtensions, filepath.Ext(path)) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if re.Match(data) {
			found = true
			return fs.SkipAll
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	return found, nil
}

// GetConversionWebhooks returns the conversion webhooks for the given package.
// It renders only the templates that declare the kind (plus helpers), and falls
// back to the full chart if that reduced render fails.
func (s *Service) GetConversionWebhooks(ctx context.Context, namespace string, pkg Package) ([]manifest.Manifest, error) {
	rendered, err := s.renderConversionWebhookChart(ctx, namespace, pkg)
	if err != nil && !errors.Is(err, ErrPackageNotHelm) {
		s.logger.Warn("render conversion webhooks chart, fallback to full render", slog.String("name", pkg.GetName()), log.Err(err))

		rendered, err = s.Render(ctx, namespace, pkg)
	}
	if err != nil {
		if errors.Is(err, ErrPackageNotHelm) {
			return nil, nil
		}
		return nil, err
	}

	all, err := manifest.ListFromYamlDocs(rendered)
	if err != nil {
		return nil, fmt.Errorf("split rendered manifests: %w", err)
	}

	webhooks := make([]manifest.Manifest, 0)
	for _, m := range all {
		if m.Kind() == conversionWebhookKind {
			webhooks = append(webhooks, m)
		}
	}

	return webhooks, nil
}

// renderConversionWebhookChart builds a reduced chart with the package's
// conversion webhook templates in a temporary directory and renders it.
func (s *Service) renderConversionWebhookChart(ctx context.Context, namespace string, pkg Package) (string, error) {
	isHelm, err := s.isHelmChart(pkg.GetPath())
	if err != nil {
		return "", fmt.Errorf("check helm chart: %w", err)
	}

	if !isHelm {
		return "", ErrPackageNotHelm
	}

	templates, err := conversionWebhookTemplates(pkg.GetPath())
	if err != nil {
		return "", fmt.Errorf("find conversion webhook templates: %w", err)
	}

	chartDir, err := os.MkdirTemp(s.tmpDir, pkg.GetName()+".conversion-webhooks-")
	if err != nil {
		return "", fmt.Errorf("create chart dir: %w", err)
	}
	defer os.RemoveAll(chartDir)

	if err = buildConversionWebhookChart(pkg.GetPath(), chartDir, templates); err != nil {
		return "", fmt.Errorf("build chart: %w", err)
	}

	valuesPath, err := s.createTmpValuesFile(pkg.GetName(), pkg.GetValues())
	if err != nil {
		return "", fmt.Errorf("create temp values file: %w", err)
	}
	defer os.Remove(valuesPath)

	return s.client.Render(ctx, namespace, pkg.GetName(), nelm.InstallOptions{
		Path:        chartDir,
		ValuesPaths: []string{valuesPath},
		RootValues:  pkg.GetRuntimeValues(),
		ResourcesLabels: map[string]string{
			health.LabelKey: pkg.GetName(),
		},
	})
}

// conversionWebhookTemplates returns paths, relative to path/templates, of the
// chart files that declare a ConversionWebhook, plus every "_" helper they may
// include (helpers render nothing on their own). A missing templates dir yields none.
// A file that only mentions the kind is left out: rendered ahead of the hooks, its
// values may not be ready yet.
func conversionWebhookTemplates(path string) ([]string, error) {
	root := filepath.Join(path, templatesDir)

	var found []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		if !strings.HasPrefix(entry.Name(), "_") {
			if !slices.Contains(chartFileExtensions, filepath.Ext(path)) {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if !conversionWebhookKindLine.Match(data) {
				return nil
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		found = append(found, rel)

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return found, nil
}

// buildConversionWebhookChart mirrors into dst the parts of the package chart
// needed to render the given templates: charts/ (library charts like helm_lib),
// webhooks/ (handlers read via .Files.Get) and the templates themselves. Directories
// are created for real and files are symlinked, so loaders that do not follow
// directory symlinks see the whole tree.
func buildConversionWebhookChart(path, dst string, templates []string) error {
	for _, dir := range []string{chartsDir, webhooksDir} {
		if err := mirrorTree(filepath.Join(path, dir), filepath.Join(dst, dir)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("mirror %s: %w", dir, err)
		}
	}

	for _, file := range templates {
		if err := linkFile(filepath.Join(path, templatesDir, file), filepath.Join(dst, templatesDir, file)); err != nil {
			return fmt.Errorf("link template '%s': %w", file, err)
		}
	}

	return nil
}

// mirrorTree recreates the src directory tree in dst following symlinks, and
// links every regular file to its resolved source.
func mirrorTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return linkFile(src, dst)
	}

	if err = os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err = mirrorTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

// linkFile symlinks dst to the resolved src, creating parent directories of dst.
func linkFile(src, dst string) error {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	return os.Symlink(resolved, dst)
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
	revision, releaseStatus, err := s.client.LastStatus(ctx, namespace, releaseName)
	if err != nil {
		return false, err
	}

	// First install - always run
	if revision == "0" {
		return true, nil
	}

	// Release exists but not deployed - need to upgrade to fix
	if strings.ToLower(releaseStatus) != "deployed" {
		return true, nil
	}

	// Check if manifests changed by comparing checksums
	recordedChecksum, err := s.client.GetChecksum(ctx, namespace, releaseName)
	if err != nil {
		// A deployed release without the checksum label was created outside this
		// runtime (e.g. by addon-operator): the recorded checksum is unknown, so
		// treat it as changed and upgrade to adopt the release and stamp the label.
		if errors.Is(err, nelm.ErrLabelNotFound) {
			return true, nil
		}

		return false, err
	}

	// Checksum changed - manifests are different, need upgrade
	if recordedChecksum != checksum {
		return true, nil
	}

	if err = s.monitorManager.CheckResources(ctx, releaseName); err != nil {
		if errors.Is(err, drift.ErrAbsentManifest) {
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

// withApplyDeadline bounds ctx by the service timeout, cancelling it with a cause
// that names the resources the apply never finished, as progress last reported them.
// The returned function ends the deadline and must be called.
func (s *Service) withApplyDeadline(ctx context.Context, progress *applyProgress) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(s.timeout, func() { cancel(applyTimeoutCause(progress.last(), s.timeout)) })

	return ctx, func() {
		timer.Stop()
		cancel(nil)
	}
}

// applyTimeoutCause is the cancellation cause for an apply that outlived the
// timeout. ops are the operations of the apply's latest progress report.
func applyTimeoutCause(ops []progrep.Operation, timeout time.Duration) error {
	waiting := waitingFor(ops)
	if len(waiting) == 0 {
		return fmt.Errorf("apply timed out after %s", timeout)
	}

	return fmt.Errorf("apply timed out after %s, waiting for %s", timeout, joinResources(waiting))
}

// waitingFor names the unfinished resources of ops as "Kind/Name". Operations
// already under way — progressing, or failed and being retried — are what an
// apply hangs on, so the queued ones are named only when there are none.
func waitingFor(ops []progrep.Operation) []string {
	started := resourcesByStatus(ops,
		progrep.OperationStatusProgressing, progrep.OperationStatusFailed)
	if len(started) > 0 {
		return started
	}

	return resourcesByStatus(ops, progrep.OperationStatusPending)
}

// resourcesByStatus renders the distinct resources of ops in one of statuses. A
// resource with several operations — an apply and a readiness track, say — is
// named once. Operations without a resource are skipped: they have no name to show.
func resourcesByStatus(ops []progrep.Operation, statuses ...progrep.OperationStatus) []string {
	resources := make([]string, 0, len(ops))
	seen := make(map[string]struct{}, len(ops))

	for _, op := range ops {
		if !isResourceOperation(op) {
			continue
		}

		if !slices.Contains(statuses, op.Status) {
			continue
		}

		resource := op.Kind + "/" + op.Name
		if _, ok := seen[resource]; ok {
			continue
		}

		seen[resource] = struct{}{}
		resources = append(resources, resource)
	}

	return resources
}

// joinResources renders at most maxNamedResources resources and counts the rest.
func joinResources(resources []string) string {
	if len(resources) <= maxNamedResources {
		return strings.Join(resources, ", ")
	}

	return fmt.Sprintf("%s and %d more",
		strings.Join(resources[:maxNamedResources], ", "), len(resources)-maxNamedResources)
}

// renderRelease renders the package's manifests; only the resource-requests path returns the
// overlaid resources, which the checksum, the absent-resources monitor and the install must all see.
func (s *Service) renderRelease(ctx context.Context, namespace string, pkg Package, opts nelm.InstallOptions) (string, []nelm.RenderedResource, error) {
	requests := resourceRequests(pkg)
	if len(requests) == 0 {
		rendered, err := s.client.Render(ctx, namespace, pkg.GetName(), opts)

		return rendered, nil, err
	}

	resized, err := s.renderResized(ctx, namespace, pkg.GetName(), opts, requests)
	if err != nil {
		return "", nil, err
	}

	rendered, err := nelm.JoinManifests(resized)
	if err != nil {
		return "", nil, err
	}

	return rendered, resized, nil
}

// observeOperation records one helm operation of a release, failed or not.
func (s *Service) observeOperation(operation, namespace, name string, started time.Time) {
	if s.metrics == nil {
		return
	}

	s.metrics.HistogramObserve(metrics.HelmOperationSeconds, time.Since(started).Seconds(), map[string]string{
		metrics.LabelModule:    name,
		metrics.LabelNamespace: namespace,
		metrics.LabelOperation: operation,
	}, nil)
}

// observeUpgrade records a whole Upgrade of a release, as addon-operator's module_helm_seconds.
func (s *Service) observeUpgrade(namespace, name string, started time.Time) {
	if s.metrics == nil {
		return
	}

	s.metrics.HistogramObserve(metrics.ModuleHelmSeconds, time.Since(started).Seconds(), map[string]string{
		metrics.LabelModule:    name,
		metrics.LabelNamespace: namespace,
	}, nil)
}
