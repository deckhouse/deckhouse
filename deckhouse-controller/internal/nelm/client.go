package nelm

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/utils"
	"github.com/werf/nelm/pkg/action"
	nelmlog "github.com/werf/nelm/pkg/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// nelmTracer is the name used for the logger instance
	nelmTracer = "nelm"

	// release label for storing checksum
	labelPackageChecksum = "packageChecksum"
)

var (
	// ErrReleaseNotFound is returned when a Helm release doesn't exist
	ErrReleaseNotFound = errors.New("release not found")
	// ErrLabelNotFound is returned when a requested label is not present in the release
	ErrLabelNotFound = errors.New("label not found")
	// ErrValuesNotFound is returned when values are not found in a release
	ErrValuesNotFound = errors.New("values not found")
)

// Options contains configuration for the nelm client
type Options struct {
	genericclioptions.ConfigFlags

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
func WithHistoryMax(max int32) Option {
	return func(o *Options) {
		o.HistoryMax = max
	}
}

// WithTimeout sets the timeout duration for Helm operations
func WithTimeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithLabels sets labels to be applied to all Kubernetes resources
func WithLabels(labels map[string]string) Option {
	return func(o *Options) {
		maps.Copy(o.Labels, labels)
	}
}

// WithAnnotations sets annotations to be applied to all Kubernetes resources
func WithAnnotations(annotations map[string]string) Option {
	return func(o *Options) {
		maps.Copy(o.Annotations, annotations)
	}
}

// Client is a wrapper around nelm operations that provides a simplified interface
type Client struct {
	opts *Options

	namespace   string
	driver      string // Helm storage driver (e.g., "secret", "configmap")
	kubeContext string

	logger *log.Logger
}

// New creates a new nelm client for the specified namespace
// It initializes the nelm logger and applies any provided options
func New(namespace string, logger *log.Logger, opts ...Option) *Client {
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

	// Build Kubernetes config flags from environment variables
	defaultOpts.ConfigFlags = *buildConfigFlagsFromEnv(namespace, cli.New())

	return &Client{
		opts: defaultOpts,

		namespace:   namespace,
		driver:      os.Getenv("HELM_DRIVER"),
		kubeContext: os.Getenv("KUBE_CONTEXT"),

		logger: logger.Named(nelmTracer),
	}
}

// buildConfigFlagsFromEnv builds Kubernetes config flags from environment variables
// Uses the Helm CLI environment settings to configure kubectl access
func buildConfigFlagsFromEnv(ns string, env *cli.EnvSettings) *genericclioptions.ConfigFlags {
	flags := genericclioptions.NewConfigFlags(true)

	// Map Helm environment settings to Kubernetes config flags
	flags.Namespace = ptr.To(ns)
	flags.Context = &env.KubeContext
	flags.BearerToken = &env.KubeToken
	flags.APIServer = &env.KubeAPIServer
	flags.CAFile = &env.KubeCaFile
	flags.KubeConfig = &env.KubeConfig
	flags.Impersonate = &env.KubeAsUser
	flags.Insecure = &env.KubeInsecureSkipTLSVerify
	flags.TLSServerName = &env.KubeTLSServerName
	flags.ImpersonateGroup = &env.KubeAsGroups
	// Apply burst limit to the rest config
	flags.WrapConfigFn = func(config *rest.Config) *rest.Config {
		config.Burst = env.BurstLimit
		return config
	}

	return flags
}

// ListCharts returns a sorted list of all chart names from installed releases
func (c *Client) ListCharts(ctx context.Context) ([]string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "ListCharts")
	defer span.End()

	res, err := action.ReleaseList(ctx, action.ReleaseListOptions{
		KubeContext:          c.kubeContext,
		OutputNoPrint:        true,
		ReleaseStorageDriver: c.driver,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("list nelm releases: %w", err)
	}

	span.SetAttributes(attribute.Int("found", len(res.Releases)))

	result := make([]string, len(res.Releases))
	for idx, release := range res.Releases {
		chartName := "unknown"
		if release.Chart != nil {
			chartName = release.Chart.Name
		}

		// Skip releases without a name
		if release.Name == "" {
			continue
		}

		result[idx] = chartName
	}

	sort.Strings(result)

	return result, nil
}

// LastStatus returns the revision number and status of the latest release
// Returns ("0", "", nil) if the release doesn't exist
func (c *Client) LastStatus(ctx context.Context, releaseName string) (string, string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "ListStatus")
	defer span.End()

	res, err := c.getRelease(ctx, releaseName)
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

// GetLabel retrieves a specific label value from a release's storage labels
func (c *Client) GetLabel(ctx context.Context, releaseName, labelName string) (string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "GetLabel")
	defer span.End()

	res, err := c.getRelease(ctx, releaseName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("get nelm release '%s': %w", releaseName, err)
	}

	if value, ok := res.Release.StorageLabels[labelName]; ok {
		return value, nil
	}

	return "", ErrLabelNotFound
}

// GetValues retrieves the values for a release and converts them to utils.Values format
// The marshal/unmarshal cycle ensures proper type conversion
func (c *Client) GetValues(ctx context.Context, releaseName string) (utils.Values, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "GetValues")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))

	res, err := c.getRelease(ctx, releaseName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get nelm release %q: %w", releaseName, err)
	}

	if res.Values == nil {
		return nil, ErrValuesNotFound
	}

	// Marshal and unmarshal to convert to utils.Values type
	raw, err := yaml.Marshal(res.Values)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("marshal values for release '%s': %w", releaseName, err)
	}

	values := make(utils.Values)
	if err = yaml.Unmarshal(raw, &values); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("unmarshal values for release '%s': %w", releaseName, err)
	}

	return values, nil
}

// GetChecksum retrieves the module checksum for a release
// It checks two locations: first the storage label "packageChecksum", then the values key "_addonOperatorModuleChecksum"
func (c *Client) GetChecksum(ctx context.Context, releaseName string) (string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "GetChecksum")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))

	res, err := c.getRelease(ctx, releaseName)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("get nelm release %q: %w", releaseName, err)
	}

	// Try to get checksum from storage labels first
	if res.Release != nil {
		if checksum, ok := res.Release.StorageLabels[labelPackageChecksum]; ok {
			return checksum, nil
		}
	}

	// Fallback to checking values for older releases
	if recordedChecksum, hasKey := res.Values["_addonOperatorModuleChecksum"]; hasKey {
		if recordedChecksumStr, ok := recordedChecksum.(string); ok {
			return recordedChecksumStr, nil
		}
	}

	return "", ErrLabelNotFound
}

// InstallOptions contains options for installing a Helm chart
type InstallOptions struct {
	Path        string   // Path to the chart directory
	ValuesPaths []string // Paths to values files
	ValuesSets  []string // Values set via --set flags

	ReleaseLabels map[string]string // Labels to apply to the release
}

// Install installs a Helm chart as a release
func (c *Client) Install(ctx context.Context, releaseName string, opts InstallOptions) error {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Install")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	extraAnnotations := make(map[string]string)
	if len(c.opts.Annotations) > 0 {
		maps.Copy(extraAnnotations, c.opts.Annotations)
	}

	// Convert maintenance label to annotation for resources
	if opts.ReleaseLabels != nil {
		maintenanceLabel, ok := opts.ReleaseLabels["maintenance.deckhouse.io/no-resource-reconciliation"]
		if ok && maintenanceLabel == "true" {
			extraAnnotations["maintenance.deckhouse.io/no-resource-reconciliation"] = ""
		}
	}

	if err := action.ReleaseInstall(ctx, releaseName, c.namespace, action.ReleaseInstallOptions{
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    "0.2.0",
		DefaultChartAPIVersion: "v2",
		ExtraLabels:            c.opts.Labels,
		ExtraAnnotations:       extraAnnotations,
		KubeContext:            c.kubeContext,
		NoInstallCRDs:          true,
		ReleaseHistoryLimit:    int(c.opts.HistoryMax),
		ReleaseLabels:          opts.ReleaseLabels,
		ReleaseStorageDriver:   c.driver,
		Timeout:                c.opts.Timeout,
		ValuesFilesPaths:       opts.ValuesPaths,
		ValuesSets:             opts.ValuesSets,
		ForceAdoption:          true,
		NoPodLogs:              true,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("install nelm release '%s': %w", releaseName, err)
	}

	return nil
}

// Render renders a Helm chart to YAML manifests without installing it
// Returns the rendered manifests as a YAML string
func (c *Client) Render(ctx context.Context, releaseName string, opts InstallOptions) (string, error) {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Render")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))
	span.SetAttributes(attribute.String("path", opts.Path))
	span.SetAttributes(attribute.String("values", strings.Join(opts.ValuesPaths, ",")))

	extraAnnotations := make(map[string]string)
	if len(c.opts.Annotations) > 0 {
		maps.Copy(extraAnnotations, c.opts.Annotations)
	}

	// Convert maintenance label to annotation for resources
	if opts.ReleaseLabels != nil {
		maintenanceLabel, ok := opts.ReleaseLabels["maintenance.deckhouse.io/no-resource-reconciliation"]
		if ok && maintenanceLabel == "true" {
			extraAnnotations["maintenance.deckhouse.io/no-resource-reconciliation"] = ""
		}
	}

	res, err := action.ChartRender(ctx, action.ChartRenderOptions{
		OutputFilePath:         "/dev/null", // No output file, we return the manifest as a string
		Chart:                  opts.Path,
		DefaultChartName:       releaseName,
		DefaultChartVersion:    "0.2.0",
		DefaultChartAPIVersion: "v2",
		ExtraLabels:            c.opts.Labels,
		ExtraAnnotations:       extraAnnotations,
		KubeContext:            c.kubeContext,
		ReleaseName:            releaseName,
		ReleaseNamespace:       c.namespace,
		ReleaseStorageDriver:   c.driver,
		Remote:                 true,
		ValuesFilesPaths:       opts.ValuesPaths,
		ValuesSets:             opts.ValuesSets,
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

// Delete uninstalls a Helm release
// Returns nil if the release doesn't exist (idempotent)
func (c *Client) Delete(ctx context.Context, releaseName string) error {
	ctx, span := otel.Tracer(nelmTracer).Start(ctx, "Delete")
	defer span.End()

	span.SetAttributes(attribute.String("release", releaseName))

	if _, err := c.getRelease(ctx, releaseName); err != nil {
		if errors.Is(err, ErrReleaseNotFound) {
			// Release doesn't exist, nothing to delete
			return nil
		}
	}

	if err := action.ReleaseUninstall(ctx, releaseName, c.namespace, action.ReleaseUninstallOptions{
		KubeContext:          c.kubeContext,
		ReleaseHistoryLimit:  int(c.opts.HistoryMax),
		ReleaseStorageDriver: c.driver,
		Timeout:              c.opts.Timeout,
		NoPodLogs:            true,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("uninstall nelm release '%s': %w", releaseName, err)
	}

	return nil
}

// getRelease is a helper method to retrieve a release by name
// Converts nelm's ReleaseNotFoundError to ErrReleaseNotFound for consistent error handling
func (c *Client) getRelease(ctx context.Context, releaseName string) (*action.ReleaseGetResultV1, error) {
	res, err := action.ReleaseGet(ctx, releaseName, c.namespace, action.ReleaseGetOptions{
		KubeContext:          c.kubeContext,
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
