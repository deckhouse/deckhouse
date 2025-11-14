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
	"maps"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/werf/nelm/pkg/action"
	"github.com/werf/nelm/pkg/common"
	nelmlog "github.com/werf/nelm/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gopkg.in/yaml.v3"

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
	nelmlog.Default = newNelmLogger(logger)

	// Set default options with history limit of 10 revisions
	defaultOpts := &Options{
		HistoryMax: 10,
	}

	// Apply any provided options
	for _, opt := range opts {
		opt(defaultOpts)
	}

	if len(defaultOpts.Annotations) == 0 {
		defaultOpts.Annotations = make(map[string]string)
	}

	defaultOpts.Annotations["werf.io/skip-logs"] = "true"
	defaultOpts.Annotations["werf.io/track-termination-mode"] = "NonBlocking"

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
	ExtraValues []byte   // Extra values in json format

	ReleaseLabels map[string]string // Labels to apply to the release
}

// Install installs a Helm chart as a release
func (c *Client) Install(ctx context.Context, namespace, releaseName string, opts InstallOptions) error {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	var valuesSet []string
	if len(opts.ExtraValues) > 0 {
		valuesSet = append(valuesSet, string(opts.ExtraValues))
	}

	if err := action.ReleaseInstall(ctx, releaseName, namespace, action.ReleaseInstallOptions{
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		ValuesOptions: common.ValuesOptions{
			ValuesFiles:    opts.ValuesPaths,
			RuntimeSetJSON: valuesSet,
		},
		TrackingOptions: common.TrackingOptions{
			NoPodLogs: true,
		},
		Chart:                   opts.Path,
		DefaultChartName:        releaseName,
		DefaultChartVersion:     "0.2.0",
		DefaultChartAPIVersion:  "v2",
		ExtraLabels:             c.opts.Labels,
		ExtraAnnotations:        c.opts.Annotations,
		NoInstallStandaloneCRDs: true,
		ReleaseHistoryLimit:     int(c.opts.HistoryMax),
		ReleaseLabels:           opts.ReleaseLabels,
		ReleaseStorageDriver:    c.driver,
		Timeout:                 c.opts.Timeout,
		ForceAdoption:           true,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install nelm release '%s': %w", releaseName, err)
	}

	return nil
}

// Render renders a nelm chart to YAML manifests without installing it
// Returns the rendered manifests as a YAML string
func (c *Client) Render(ctx context.Context, namespace, releaseName string, opts InstallOptions) (string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Render")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	var valuesSet []string
	if len(opts.ExtraValues) > 0 {
		valuesSet = append(valuesSet, string(opts.ExtraValues))
	}

	res, err := action.ChartRender(ctx, action.ChartRenderOptions{
		KubeConnectionOptions: common.KubeConnectionOptions{
			KubeContextCurrent: c.kubeContext,
		},
		ValuesOptions: common.ValuesOptions{
			ValuesFiles:    opts.ValuesPaths,
			RuntimeSetJSON: valuesSet,
		},
		OutputFilePath:         "/dev/null", // No output file, we return the manifest as a string
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    "0.2.0",
		DefaultChartAPIVersion: "v2",
		ExtraLabels:            c.opts.Labels,
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
		marshalled, err := yaml.Marshal(resource)
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
func (c *Client) Delete(ctx context.Context, namespace, releaseName string) error {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Delete")
	defer span.End()

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
func (c *Client) getRelease(ctx context.Context, namespace, releaseName string) (*action.ReleaseGetResultV1, error) {
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
