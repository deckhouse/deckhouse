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
	"maps"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/werf/nelm/pkg/action"
	"github.com/werf/nelm/pkg/common"
	"github.com/werf/nelm/pkg/legacy/progrep"
	nelmlog "github.com/werf/nelm/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// logger and telemetry name
	nelmTracer = "nelm"

	// LabelPackageChecksum release label for storing checksum
	LabelPackageChecksum = "packageChecksum"
)

var (
	// ErrReleaseNotFound is returned when a nelm release doesn't exist
	ErrReleaseNotFound = errors.New("release not found")
	// ErrLabelNotFound is returned when a requested label is not present in the release
	ErrLabelNotFound = errors.New("label not found")

	one sync.Once
)

// Options contains configuration for the nelm client
type Options struct {
	// HistoryMax defines the maximum number of release revisions to keep
	HistoryMax int32
	// Timeout for Helm operations
	Timeout time.Duration

	// Labels to apply to Kubernetes resources
	Labels map[string]string
	// Annotations to apply to Kubernetes resources
	Annotations map[string]string
}

// Option is a functional option for configuring the nelm client
type Option func(*Options)

// WithHistoryMax sets the maximum number of release revisions to keep in history
func WithHistoryMax(historyMax int32) Option {
	return func(o *Options) {
		o.HistoryMax = historyMax
	}
}

// WithTimeout sets the timeout duration for nelm operations
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithLabels sets labels to be applied to all releases
func WithLabels(labels map[string]string) Option {
	return func(o *Options) {
		maps.Copy(o.Labels, labels)
	}
}

// WithAnnotations sets annotations to be applied to all releases
func WithAnnotations(annotations map[string]string) Option {
	return func(o *Options) {
		maps.Copy(o.Annotations, annotations)
	}
}

// Client is a wrapper around nelm operations that provides a simplified interface
type Client struct {
	opts *Options

	driver      string // Helm storage driver (e.g., "secret", "configmap")
	kubeContext string

	logger *log.Logger
}

// New creates a new nelm client for the specified namespace
// It initializes the nelm logger and applies any provided options
func New(logger *log.Logger, opts ...Option) *Client {
	// Set the default nelm logger to our custom adapter
	one.Do(func() {
		nelmlog.Default = newNelmLogger(logger)
	})

	// Set default options with history limit of 10 revisions
	defaultOpts := &Options{
		HistoryMax:  10,
		Annotations: make(map[string]string),
		Labels:      make(map[string]string),
		Timeout:     2 * time.Minute,
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(defaultOpts)
	}

	return &Client{
		opts: defaultOpts,

		driver:      os.Getenv("HELM_DRIVER"),
		kubeContext: os.Getenv("KUBE_CONTEXT"),

		logger: logger.Named(nelmTracer),
	}
}

// LastStatus returns the revision number and status of the latest release
// Returns ("0", "", nil) if the release doesn't exist
func (c *Client) LastStatus(ctx context.Context, namespace, releaseName string) (string, string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "ListStatus")
	defer span.End()

	res, err := c.getRelease(ctx, namespace, releaseName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		if errors.Is(err, ErrReleaseNotFound) {
			// Return zero revision for non-existent releases
			return "0", "", nil
		}

		return "", "", err
	}

	return strconv.FormatInt(int64(res.Release.Revision), 10), res.Release.Status.String(), nil
}

// GetChecksum retrieves the module checksum for a release
// It checks the storage label "packageChecksum"
func (c *Client) GetChecksum(ctx context.Context, namespace, releaseName string) (string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "GetChecksum")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))

	res, err := c.getRelease(ctx, namespace, releaseName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("get nelm release '%s': %w", releaseName, err)
	}

	// Try to get checksum from storage labels first
	if res.Release != nil {
		if checksum, ok := res.Release.StorageLabels[LabelPackageChecksum]; ok {
			return checksum, nil
		}
	}

	return "", ErrLabelNotFound
}

// InstallOptions contains options for installing a Helm chart
type InstallOptions struct {
	Path        string   // Path to the chart directory
	ValuesPaths []string // Paths to values files
	RootValues  string   // Values in JSON format

	ReleaseLabels map[string]string // Labels to apply to the release

	ResourcesLabels map[string]string // Labels to apply to all resources

	// OnTrackingEvent is an optional callback invoked with progress updates
	// as Kubernetes resources are being tracked for readiness during install.
	OnTrackingEvent func(name string, report progrep.ProgressReport)
}

// Install installs a Helm chart as a release
//
//nolint:nonamedreturns // named returns required for defer/recover to modify return values
func (c *Client) Install(ctx context.Context, namespace, releaseName string, opts InstallOptions) (err error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Install")
	defer span.End()
	defer c.recoverPanic("Install", span, &err,
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
		slog.String("path", opts.Path),
	)

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	var valuesSet []string
	if len(opts.RootValues) > 0 {
		valuesSet = append(valuesSet, opts.RootValues)
	}

	labels := maps.Clone(c.opts.Labels)
	if len(opts.ResourcesLabels) > 0 {
		if labels == nil {
			labels = make(map[string]string, len(opts.ResourcesLabels))
		}
		maps.Copy(labels, opts.ResourcesLabels)
	}

	// reportCh receives progress reports from nelm during resource tracking.
	// A background goroutine converts each report into a tracking event and
	// forwards it to the caller's callback. The channel is closed when the
	// install operation completes.
	reportCh := make(chan progrep.ProgressReport, 1)
	defer close(reportCh)

	go func() {
		for report := range reportCh {
			if opts.OnTrackingEvent != nil {
				opts.OnTrackingEvent(releaseName, report)
			}
		}
	}()

	if err := action.ReleaseInstall(ctx, releaseName, namespace, action.ReleaseInstallOptions{
		LegacyProgressReportCh: reportCh,
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		ValuesOptions: common.ValuesOptions{
			ValuesFiles: opts.ValuesPaths,
			RootSetJSON: valuesSet,
		},
		TrackingOptions: common.TrackingOptions{
			NoPodLogs: true,
		},
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    "0.2.0",
		DefaultChartAPIVersion: "v2",
		ReleaseInstallRuntimeOptions: common.ReleaseInstallRuntimeOptions{
			ExtraLabels:             labels,
			ExtraAnnotations:        c.opts.Annotations,
			NoInstallStandaloneCRDs: true,
			ReleaseHistoryLimit:     int(c.opts.HistoryMax),
			ReleaseLabels:           opts.ReleaseLabels,
			ReleaseStorageDriver:    c.driver,
			ForceAdoption:           true,
		},
		Timeout: c.opts.Timeout,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install nelm release '%s': %w", releaseName, err)
	}

	return nil
}

// Render renders a nelm chart to YAML manifests without installing it
// Returns the rendered manifests as a YAML string
//
//nolint:nonamedreturns // named returns required for defer/recover to modify return values
func (c *Client) Render(ctx context.Context, namespace, releaseName string, opts InstallOptions) (out string, err error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Render")
	defer span.End()
	defer c.recoverPanic("Render", span, &err,
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
		slog.String("path", opts.Path),
	)

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	var valuesSet []string
	if len(opts.RootValues) > 0 {
		valuesSet = append(valuesSet, opts.RootValues)
	}

	labels := maps.Clone(c.opts.Labels)
	if len(opts.ResourcesLabels) > 0 {
		if labels == nil {
			labels = make(map[string]string, len(opts.ResourcesLabels))
		}
		maps.Copy(labels, opts.ResourcesLabels)
	}

	res, err := action.ChartRender(ctx, action.ChartRenderOptions{
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		ValuesOptions: common.ValuesOptions{
			ValuesFiles: opts.ValuesPaths,
			RootSetJSON: valuesSet,
		},
		OutputFilePath:         "/dev/null", // No output file, we return the manifest as a string
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    "0.2.0",
		DefaultChartAPIVersion: "v2",
		ExtraLabels:            labels,
		ExtraAnnotations:       c.opts.Annotations,
		ReleaseName:            releaseName,
		ReleaseNamespace:       namespace,
		ReleaseStorageDriver:   c.driver,
		Remote:                 true,
		ForceAdoption:          true,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("render nelm chart '%s': %w", opts.Path, err)
	}

	// Combine all resources into a single YAML document with separators
	var result strings.Builder
	for _, resource := range res.Resources {
		marshalled, err := yaml.Marshal(resource.Unstruct)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return "", fmt.Errorf("marshal resource: %w", err)
		}

		if result.Len() > 0 {
			result.WriteString("---\n")
		}

		result.Write(marshalled)
	}

	return result.String(), nil
}

// Delete uninstalls a nelm release
// Returns nil if the release doesn't exist (idempotent)
//
//nolint:nonamedreturns // named returns required for defer/recover to modify return values
func (c *Client) Delete(ctx context.Context, namespace, releaseName string) (err error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Delete")
	defer span.End()
	defer c.recoverPanic("Delete", span, &err,
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
	)

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))

	if _, err := c.getRelease(ctx, namespace, releaseName); err != nil {
		if errors.Is(err, ErrReleaseNotFound) {
			// Release doesn't exist, nothing to delete
			return nil
		}
	}

	if err := action.ReleaseUninstall(ctx, releaseName, namespace, action.ReleaseUninstallOptions{
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		TrackingOptions: common.TrackingOptions{
			NoPodLogs: true,
		},
		ReleaseHistoryLimit:  int(c.opts.HistoryMax),
		ReleaseStorageDriver: c.driver,
		Timeout:              c.opts.Timeout,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("uninstall nelm release '%s': %w", releaseName, err)
	}

	return nil
}

// getRelease is a helper method to retrieve a release by name
// Converts nelm's ReleaseNotFoundError to ErrReleaseNotFound for consistent error handling
//
//nolint:nonamedreturns // named returns required for defer/recover to modify return values
func (c *Client) getRelease(ctx context.Context, namespace, releaseName string) (result *action.ReleaseGetResultV1, err error) {
	defer c.recoverPanic("getRelease", nil, &err,
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
	)

	res, err := action.ReleaseGet(ctx, releaseName, namespace, action.ReleaseGetOptions{
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		OutputNoPrint:        true,
		ReleaseStorageDriver: c.driver,
	})
	if err != nil {
		var releaseNotFoundErr *action.ReleaseNotFoundError
		if errors.As(err, &releaseNotFoundErr) {
			// Convert to our standard error type
			return nil, ErrReleaseNotFound
		}

		return nil, fmt.Errorf("get nelm release '%s': %w", releaseName, err)
	}

	return res, nil
}

// recoverPanic converts a panic recovered from a deferred call into err,
// logs it with stack trace and the supplied context fields, and marks span
// (when non-nil) as failed. Must be deferred directly so recover() works.
func (c *Client) recoverPanic(op string, span trace.Span, err *error, fields ...slog.Attr) {
	r := recover()
	if r == nil {
		return
	}

	args := make([]any, 0, len(fields)+2)
	args = append(args, slog.Any("panic", r))
	for _, f := range fields {
		args = append(args, f)
	}
	args = append(args, slog.String("stack", string(debug.Stack())))
	c.logger.Error("panic in "+op, args...)

	*err = fmt.Errorf("panic in %s: %v", op, r)
	if span != nil {
		span.SetStatus(codes.Error, (*err).Error())
	}
}
