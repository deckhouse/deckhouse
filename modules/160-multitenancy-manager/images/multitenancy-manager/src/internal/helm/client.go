/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/go-logr/logr"
	"github.com/go-openapi/spec"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/validate"
)

const (
	helmDriver = "secret"

	ResourceAnnotationReleaseName      = "meta.helm.sh/release-name"
	ResourceAnnotationReleaseNamespace = "meta.helm.sh/release-namespace"

	ResourceLabelManagedBy = "app.kubernetes.io/managed-by"
)

// structs.DefaultTagName is a process-wide global in fatih/structs. Set it once at package load so
// buildValues stays read-only and free of data races under concurrent reconciles.
func init() {
	structs.DefaultTagName = "yaml"
}

type Client struct {
	conf      *action.Configuration
	templates map[string][]byte
	opts      *options
	logger    logr.Logger
}

type options struct {
	HistoryMax int32
	Timeout    time.Duration
}

// New initializes helm client with secret backend storage in `namespace` arg namespace.
func New(namespace, templatesPath string, logger logr.Logger) (*Client, error) {
	cli := &Client{
		opts: &options{
			HistoryMax: 3,
			Timeout:    15 * time.Second,
		},
		conf: &action.Configuration{
			Capabilities: chartutil.DefaultCapabilities,
		},
		logger:    logger.WithName("helm"),
		templates: make(map[string][]byte),
	}

	cli.logger.Info("initializing action config")
	if err := cli.initActionConfig(namespace); err != nil {
		return nil, fmt.Errorf("initialize action config: %w", err)
	}

	var err error
	cli.templates, err = parseHelmTemplates(templatesPath)
	if err != nil {
		return nil, fmt.Errorf("parse helm templates: %w", err)
	}

	cli.logger.Info("client initialized")
	return cli, nil
}

func parseHelmTemplates(templatesPath string) (map[string][]byte, error) {
	helmTemplates := make(map[string][]byte)
	dir, err := os.ReadDir(templatesPath)
	if err != nil {
		return nil, err
	}
	for _, file := range dir {
		if file.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(templatesPath, file.Name()))
		if err != nil {
			return nil, err
		}
		helmTemplates[file.Name()] = data
	}
	return helmTemplates, nil
}

func (c *Client) initActionConfig(namespace string) error {
	// create the rest config instance with ServiceAccount values loaded in them
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("initialize cluster rest config: %w", err)
	}

	// create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace

	return c.conf.Init(kubeConfig, namespace, helmDriver, c.DebugLog)
}

func (c *Client) DebugLog(format string, args ...any) {
	c.logger.Info(fmt.Sprintf(format, args...))
}

// ReleaseOutcome reports the result of a release attempt (or a standalone manifest analysis). Filtered
// and RoleRefs are pure functions of the rendered manifests (whether a controller-managed
// ResourceQuota/AuthorizationRule was dropped, and the roleRefs of every rendered binding). Applied is
// true when the release was (re)installed/upgraded this call and the post-renderer therefore populated
// Filtered/RoleRefs; it is false when the release was already up to date (no post-render ran), in which
// case the caller must analyze the manifests separately to obtain Filtered/RoleRefs.
type ReleaseOutcome struct {
	Filtered bool
	RoleRefs []BindingRoleRef
	Applied  bool
}

// Upgrade renders a legacy resourcesTemplate (helm-string) template and installs/upgrades the project
// release from it.
func (c *Client) Upgrade(ctx context.Context, project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (ReleaseOutcome, error) {
	values := buildValues(project, template)
	return c.release(ctx, project, buildChart(c.templates, project.Name), values, hashMD5(c.templates, values), "")
}

// UpgradeManifests installs/upgrades the project release from manifests rendered natively from a
// schema-based (v1alpha2) ProjectTemplate. The Helm chart is a no-op (empty) template: the concrete
// objects are supplied to the post-renderer, so no user data passes through the Helm template engine
// while Helm still drives the release lifecycle (install/upgrade/prune/history). The hash is taken
// over the rendered manifests so a structural or parameter change re-applies the release.
func (c *Client) UpgradeManifests(ctx context.Context, project *v1alpha3.Project, manifests string) (ReleaseOutcome, error) {
	return c.release(ctx, project, buildEmptyChart(project.Name), map[string]any{}, hashString(manifests), manifests)
}

// release runs the shared install/upgrade machinery: discovery, history lookup, up-to-date short
// circuit, pending-release rollback and the post-renderer. manifestsOverride is empty for the legacy
// helm-string path and set to the natively rendered objects for the schema-based path. On an
// up-to-date release it short-circuits WITHOUT post-rendering and returns Applied=false, so the caller
// must analyze the manifests to recover Filtered/RoleRefs.
func (c *Client) release(ctx context.Context, project *v1alpha3.Project, ch *chart.Chart, values map[string]any, hash, manifestsOverride string) (ReleaseOutcome, error) {
	versions, err := c.discoverAPI()
	if err != nil {
		return ReleaseOutcome{}, fmt.Errorf("discover api: %w", err)
	}

	// The Helm release name is derived from the project name: it equals the project name when it fits
	// Helm's 53-char limit and is deterministically shortened otherwise (see releaseName). The project
	// namespace stays the raw project name.
	rel := releaseName(project.Name)

	// action.History exposes no context or timeout; the lookup is a single, bounded API read.
	releases, err := action.NewHistory(c.conf).Run(rel)
	isFirstInstall := false
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			isFirstInstall = true
			c.logger.Info("the release not found, install it", "release", rel, "namespace", project.Name)
			post := newPostRenderer(project, versions, c.logger, isFirstInstall)
			post.manifests = manifestsOverride
			install := action.NewInstall(c.conf)
			install.ReleaseName = rel
			install.Timeout = c.opts.Timeout
			install.UseReleaseName = true
			install.Labels = map[string]string{
				v1alpha3.ReleaseLabelHashsum: hash,
			}
			install.PostRenderer = post
			if _, err = install.RunWithContext(ctx, ch, values); err != nil {
				return ReleaseOutcome{}, fmt.Errorf("install the release: %w", err)
			}
			c.logger.Info("the release installed", "release", rel, "namespace", project.Name)
			return ReleaseOutcome{Filtered: post.filtered, RoleRefs: post.referencedRoles, Applied: true}, nil
		}
		return ReleaseOutcome{}, fmt.Errorf("retrieve history for the release: %w", err)
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	if releaseHash, ok := releases[0].Labels[v1alpha3.ReleaseLabelHashsum]; ok {
		if releaseHash == hash && releases[0].Info.Status == release.StatusDeployed {
			c.logger.Info("the release is up to date", "release", rel, "namespace", project.Name)
			return ReleaseOutcome{Applied: false}, nil
		}
	}

	if releases[0].Info.Status.IsPending() {
		if err = c.rollbackLatestRelease(releases); err != nil {
			return ReleaseOutcome{}, fmt.Errorf("rollback latest release: %w", err)
		}
	}

	post := newPostRenderer(project, versions, c.logger, isFirstInstall)
	post.manifests = manifestsOverride
	upgrade := action.NewUpgrade(c.conf)
	upgrade.Install = true
	upgrade.MaxHistory = int(c.opts.HistoryMax)
	upgrade.Timeout = c.opts.Timeout
	upgrade.Labels = map[string]string{
		v1alpha3.ReleaseLabelHashsum: hash,
	}
	upgrade.PostRenderer = post

	if _, err = upgrade.RunWithContext(ctx, rel, ch, values); err != nil {
		return ReleaseOutcome{}, fmt.Errorf("upgrade the release: %w", err)
	}

	c.logger.Info("the release upgraded", "release", rel, "namespace", project.Name)
	return ReleaseOutcome{Filtered: post.filtered, RoleRefs: post.referencedRoles, Applied: true}, nil
}

// discoverAPI returns api versions, they will be used in the post renderer. The discovery client API
// (ServerGroupsAndResources) accepts neither a context nor a deadline, so this call cannot be bound
// by ctx; it is limited only by the REST client's own transport timeout.
func (c *Client) discoverAPI() (map[string]struct{}, error) {
	dc, err := c.conf.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("get discovery client: %w", err)
	}

	dc.Invalidate()

	var resources []*metav1.APIResourceList
	if _, resources, err = dc.ServerGroupsAndResources(); err != nil {
		return nil, fmt.Errorf("discover api: %w", err)
	}

	versions := make(map[string]struct{})
	for _, resourcesList := range resources {
		for _, resource := range resourcesList.APIResources {
			versions[filepath.Join(resourcesList.GroupVersion, resource.Kind)] = struct{}{}
		}
	}

	return versions, nil
}

// helmReleaseNameMaxLen is Helm's hard limit on release names (helm.sh/helm/v3/pkg/action rejects
// longer names). The project CRD allows names up to 61 chars (a project name is also a namespace,
// capped at 63), so a project name may exceed this limit and must be mapped to a valid release name.
const helmReleaseNameMaxLen = 53

// releaseName maps a project name to its Helm release name. A name within Helm's limit is used
// verbatim so existing releases keep their names (no migration); a longer name is deterministically
// shortened to a collision-resistant form (a truncated prefix plus an 8-hex md5 suffix of the full
// name) that stays a valid, unique release name. The project namespace remains the raw project name.
func releaseName(project string) string {
	if len(project) <= helmReleaseNameMaxLen {
		return project
	}
	const suffixLen = 9 // '-' + 8 hex characters
	prefix := strings.TrimRight(project[:helmReleaseNameMaxLen-suffixLen], "-")
	return prefix + "-" + hashString(project)[:8]
}

func buildChart(templates map[string][]byte, releaseName string) *chart.Chart {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    releaseName,
			Version: "0.0.1",
		},
	}

	for name, template := range templates {
		if !strings.HasPrefix(name, "templates/") {
			name = "templates/" + name
		}
		chartFile := chart.File{
			Name: name,
			Data: template,
		}
		ch.Templates = append(ch.Templates, &chartFile)
	}

	return ch
}

// buildEmptyChart builds a chart whose only template renders to nothing. It is used by the
// schema-based render path: the real objects are supplied to the post-renderer, so the chart exists
// only to drive Helm's release machinery and must not template any user data.
func buildEmptyChart(releaseName string) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    releaseName,
			Version: "0.0.1",
		},
		Templates: []*chart.File{
			{
				Name: "templates/manifests.yaml",
				Data: []byte("# rendered natively by the controller; see internal/render\n"),
			},
		},
	}
}

func buildValues(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) map[string]any {
	// Work on a copy of the spec so the empty-template placeholder below does not mutate the
	// caller's template (a cache-shared object reused across reconciles).
	templateSpec := template.Spec
	if len(templateSpec.ResourcesTemplate) == 0 {
		templateSpec.ResourcesTemplate = " "
	}

	// skip error, invalid template cannot be here due to validation
	schema, _ := validate.LoadSchema(templateSpec.ParametersSchema.OpenAPIV3Schema)

	preparedProject := struct {
		Name         string         `json:"projectName" yaml:"projectName"`
		TemplateName string         `json:"projectTemplateName" yaml:"projectTemplateName"`
		Parameters   map[string]any `json:"parameters" yaml:"parameters"`
	}{
		Name:         project.Name,
		TemplateName: project.Spec.ProjectTemplateName,
		Parameters:   mergeWithDefaults(schema, project.Spec.Parameters),
	}

	return map[string]any{
		"projectTemplate": structs.Map(templateSpec),
		"project":         structs.Map(preparedProject),
	}
}

// mergeWithDefaults overlays the schema's property defaults onto the project values. It delegates to
// validate.MergeDefaults, the single implementation shared with the structured render path; it is kept
// as a thin wrapper so the helm package tests (TestMergeWithDefaults_*) keep their entry point.
func mergeWithDefaults(schema *spec.Schema, projectValues map[string]any) map[string]any {
	return validate.MergeDefaults(schema, projectValues)
}

func (c *Client) rollbackLatestRelease(releases []*release.Release) error {
	latestRelease := releases[0]

	if latestRelease.Version == 1 || c.opts.HistoryMax == 1 || len(releases) == 1 {
		uninstall := action.NewUninstall(c.conf)
		uninstall.KeepHistory = false
		_, err := uninstall.Run(latestRelease.Name)
		return err
	}

	previousVersion := latestRelease.Version - 1
	for i := 1; i < len(releases); i++ {
		if !releases[i].Info.Status.IsPending() {
			previousVersion = releases[i].Version
			break
		}
	}

	rollback := action.NewRollback(c.conf)
	rollback.Version = previousVersion
	rollback.CleanupOnFail = true

	return rollback.Run(latestRelease.Name)
}

// Delete deletes resources. The Helm uninstall API does not accept a context, so ctx is honoured by
// bailing out before starting a teardown that is already cancelled, and an explicit Timeout bounds
// the uninstall wait (mirroring the install/upgrade timeout) so a slow API server cannot stall it.
func (c *Client) Delete(ctx context.Context, projectName string) error {
	rel := releaseName(projectName)
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("uninstall the '%s' release: %w", rel, err)
	}

	uninstall := action.NewUninstall(c.conf)
	uninstall.KeepHistory = false
	uninstall.IgnoreNotFound = true
	uninstall.Timeout = c.opts.Timeout

	if _, err := uninstall.Run(rel); err != nil {
		return fmt.Errorf("uninstall the '%s' release: %w", rel, err)
	}

	c.logger.Info("the release deleted", "release", rel)
	return nil
}

// AnalyzeRendered renders the legacy project template and returns the filtered flag plus the roleRefs
// of every binding object it declares (RoleBinding, ClusterRoleBinding, ProjectRoleBinding,
// ClusterProjectRoleBinding). Both are pure functions of the rendered manifests, so this is used on
// the up-to-date path where the release apply short-circuits without post-rendering. It runs on a copy
// of the project so it does not mutate the caller's status.
func (c *Client) AnalyzeRendered(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (ReleaseOutcome, error) {
	ch := buildChart(c.templates, project.Name)

	values, err := chartutil.ToRenderValues(ch, buildValues(project, template), chartutil.ReleaseOptions{
		Name:      releaseName(project.Name),
		Namespace: project.Name,
	}, nil)
	if err != nil {
		return ReleaseOutcome{}, fmt.Errorf("render values: %w", err)
	}

	rendered, err := engine.Render(ch, values)
	if err != nil {
		return ReleaseOutcome{}, fmt.Errorf("render chart: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	renderer := newPostRenderer(project.DeepCopy(), nil, c.logger, false)
	if _, err = renderer.Run(buf); err != nil {
		return ReleaseOutcome{}, fmt.Errorf("post render: %w", err)
	}

	return ReleaseOutcome{Filtered: renderer.filtered, RoleRefs: renderer.referencedRoles}, nil
}

// AnalyzeManifests returns the filtered flag and binding roleRefs of natively rendered manifests (the
// schema-based equivalent of AnalyzeRendered). It runs the post-renderer on a copy of the project so
// it does not mutate the caller's status.
func (c *Client) AnalyzeManifests(project *v1alpha3.Project, manifests string) (ReleaseOutcome, error) {
	renderer := newPostRenderer(project.DeepCopy(), nil, c.logger, false)
	renderer.manifests = manifests
	if _, err := renderer.Run(bytes.NewBuffer(nil)); err != nil {
		return ReleaseOutcome{}, fmt.Errorf("post render: %w", err)
	}
	return ReleaseOutcome{Filtered: renderer.filtered, RoleRefs: renderer.referencedRoles}, nil
}

// ValidateManifests checks natively rendered manifests for the namespace-override warning (the
// schema-based equivalent of ValidateRender).
func (c *Client) ValidateManifests(project *v1alpha3.Project, manifests string) error {
	renderer := newPostRenderer(project, nil, c.logger, false)
	renderer.manifests = manifests
	if _, err := renderer.Run(bytes.NewBuffer(nil)); err != nil {
		return fmt.Errorf("post render: %w", err)
	}
	return renderer.warning
}

// ValidateRender tests project render
func (c *Client) ValidateRender(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) error {
	ch := buildChart(c.templates, project.Name)

	values, err := chartutil.ToRenderValues(ch, buildValues(project, template), chartutil.ReleaseOptions{
		Name:      releaseName(project.Name),
		Namespace: project.Name,
	}, nil)
	if err != nil {
		return fmt.Errorf("render values: %w", err)
	}

	rendered, err := engine.Render(ch, values)
	if err != nil {
		return fmt.Errorf("render chart: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	renderer := newPostRenderer(project, nil, c.logger, false)
	if _, err = renderer.Run(buf); err != nil {
		return fmt.Errorf("post render: %w", err)
	}

	return renderer.warning
}
