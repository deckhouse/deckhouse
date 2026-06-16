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

// Upgrade upgrades resources. It returns whether any controller-managed resource (ResourceQuota,
// AuthorizationRule) was filtered out of the rendered template.
func (c *Client) Upgrade(ctx context.Context, project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) (bool, error) {
	ch := buildChart(c.templates, project.Name)

	versions, err := c.discoverAPI()
	if err != nil {
		return false, fmt.Errorf("discover api: %w", err)
	}

	values := buildValues(project, template)
	hash := hashMD5(c.templates, values)

	// action.History exposes no context or timeout; the lookup is a single, bounded API read.
	releases, err := action.NewHistory(c.conf).Run(project.Name)
	isFirstInstall := false
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			isFirstInstall = true
			c.logger.Info("the release not found, install it", "release", project.Name, "namespace", project.Name)
			post := newPostRenderer(project, versions, c.logger, isFirstInstall)
			install := action.NewInstall(c.conf)
			install.ReleaseName = project.Name
			install.Timeout = c.opts.Timeout
			install.UseReleaseName = true
			install.Labels = map[string]string{
				v1alpha3.ReleaseLabelHashsum: hash,
			}
			install.PostRenderer = post
			if _, err = install.RunWithContext(ctx, ch, values); err != nil {
				return false, fmt.Errorf("install the release: %w", err)
			}
			c.logger.Info("the release installed", "release", project.Name, "namespace", project.Name)
			return post.filtered, nil
		}
		return false, fmt.Errorf("retrieve history for the release: %w", err)
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	if releaseHash, ok := releases[0].Labels[v1alpha3.ReleaseLabelHashsum]; ok {
		if releaseHash == hash && releases[0].Info.Status == release.StatusDeployed {
			c.logger.Info("the release is up to date", "release", project.Name, "namespace", project.Name)
			return false, nil
		}
	}

	if releases[0].Info.Status.IsPending() {
		if err = c.rollbackLatestRelease(releases); err != nil {
			return false, fmt.Errorf("rollback latest release: %w", err)
		}
	}

	post := newPostRenderer(project, versions, c.logger, isFirstInstall)
	upgrade := action.NewUpgrade(c.conf)
	upgrade.Install = true
	upgrade.MaxHistory = int(c.opts.HistoryMax)
	upgrade.Timeout = c.opts.Timeout
	upgrade.Labels = map[string]string{
		v1alpha3.ReleaseLabelHashsum: hash,
	}
	upgrade.PostRenderer = post

	if _, err = upgrade.RunWithContext(ctx, project.Name, ch, values); err != nil {
		return false, fmt.Errorf("upgrade the release: %w", err)
	}

	c.logger.Info("the release upgraded", "release", project.Name, "namespace", project.Name)
	return post.filtered, nil
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

func mergeWithDefaults(schema *spec.Schema, projectValues map[string]any) map[string]any {
	result := make(map[string]any)

	for property, propertySchema := range schema.Properties {
		if projectValue, exists := projectValues[property]; exists {
			// use project value
			result[property] = projectValue

			// recursively handle nested objects
			if propertySchema.Type.Contains("object") {
				if valueMap, ok := projectValue.(map[string]any); ok {
					result[property] = mergeWithDefaults(&propertySchema, valueMap)
				}
			}
		} else if propertySchema.Default != nil {
			// use default value from schema
			result[property] = propertySchema.Default
		}

		// recursively apply defaults to nested objects
		if propertySchema.Type.Contains("object") {
			if _, ok := result[property]; !ok {
				result[property] = mergeWithDefaults(&propertySchema, nil)
			}
		}
	}

	// Handle additionalProperties (free-form map types). When a schema declares additionalProperties
	// it models a map, not a fixed object, so the values are user-controlled keys. We deliberately
	// REPLACE result here (discarding the property defaults computed above): an additionalProperties
	// schema and named-property defaults are not combined, the map's own keys win. This branch is
	// pinned by TestMergeWithDefaults_AdditionalProperties.
	if schema.AdditionalProperties != nil {
		mapResult := make(map[string]any)
		for key, value := range projectValues {
			// skip keys that are already handled as properties
			if _, exists := schema.Properties[key]; exists {
				continue
			}

			mapResult[key] = value
		}

		result = mapResult
	}

	return result
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
func (c *Client) Delete(ctx context.Context, releaseName string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("uninstall the '%s' release: %w", releaseName, err)
	}

	uninstall := action.NewUninstall(c.conf)
	uninstall.KeepHistory = false
	uninstall.IgnoreNotFound = true
	uninstall.Timeout = c.opts.Timeout

	if _, err := uninstall.Run(releaseName); err != nil {
		return fmt.Errorf("uninstall the '%s' release: %w", releaseName, err)
	}

	c.logger.Info("the release deleted", "release", releaseName)
	return nil
}

// RenderedRoleRefs renders the project template and returns the roleRefs of every binding object it
// declares (RoleBinding, ClusterRoleBinding, ProjectRoleBinding, ClusterProjectRoleBinding). It runs
// on a copy of the project so collecting the refs does not mutate the caller's status.
func (c *Client) RenderedRoleRefs(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) ([]BindingRoleRef, error) {
	ch := buildChart(c.templates, project.Name)

	values, err := chartutil.ToRenderValues(ch, buildValues(project, template), chartutil.ReleaseOptions{
		Name:      project.Name,
		Namespace: project.Name,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("render values: %w", err)
	}

	rendered, err := engine.Render(ch, values)
	if err != nil {
		return nil, fmt.Errorf("render chart: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	renderer := newPostRenderer(project.DeepCopy(), nil, c.logger, false)
	if _, err = renderer.Run(buf); err != nil {
		return nil, fmt.Errorf("post render: %w", err)
	}

	return renderer.referencedRoles, nil
}

// ValidateRender tests project render
func (c *Client) ValidateRender(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) error {
	ch := buildChart(c.templates, project.Name)

	values, err := chartutil.ToRenderValues(ch, buildValues(project, template), chartutil.ReleaseOptions{
		Name:      project.Name,
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
