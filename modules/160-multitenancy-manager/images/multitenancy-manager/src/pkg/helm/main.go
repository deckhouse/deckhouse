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

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"

	"github.com/fatih/structs"

	"github.com/go-logr/logr"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Client struct {
	conf      *action.Configuration
	templates map[string][]byte
	opts      *options
	log       logr.Logger
}

type options struct {
	HistoryMax int32
	Timeout    time.Duration
	DryRun     bool
}

type Option func(options *options)

func WithHistoryMax(historyMax int32) Option {
	return func(options *options) {
		options.HistoryMax = historyMax
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(options *options) {
		options.Timeout = timeout
	}
}

// New initializes helm client with secret backend storage in `namespace` arg namespace.
// Possible options:
// WithHistoryMax - set maximum stored releases. Default: 3
// WithTimeout - timeout for helm upgrade/delete. Default: 15 seconds
func New(namespace, templatesPath string, log logr.Logger, opts ...Option) (*Client, error) {
	c := &Client{
		opts: &options{
			HistoryMax: 3,
			Timeout:    time.Duration(15 * float64(time.Second)),
		},
		conf: &action.Configuration{
			Capabilities: chartutil.DefaultCapabilities,
		},
		log:       log.WithName("helm"),
		templates: make(map[string][]byte),
	}

	for _, opt := range opts {
		opt(c.opts)
	}

	c.log.Info("initializing action config")
	if err := c.initActionConfig(namespace); err != nil {
		c.log.Error(err, "failed to initialize action config")
		return nil, err
	}

	templates, err := parseHelmTemplates(templatesPath)
	if err != nil {
		c.log.Error(err, "failed to parse helm templates")
		return nil, err
	}
	c.templates = templates

	c.log.Info("client initialized")
	return c, nil
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
		c.log.Error(err, "failed to initialize cluster rest config")
		return err
	}

	// create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig := genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace

	return c.conf.Init(kubeConfig, namespace, consts.HelmDriver, klog.Infof)
}

// Upgrade upgrades resources
func (c *Client) Upgrade(ctx context.Context, project *v1alpha2.Project, template *v1alpha1.ProjectTemplate) error {
	ch, err := makeChart(c.templates, project.Name)
	if err != nil {
		c.log.Error(err, "failed to make chart", "release", project.Name, "namespace", project.Name)
		return err
	}

	values := buildValues(project, template)
	hash := hashMD5(c.templates, values)
	post := newPostRenderer(project.Name, template.Name, c.log)

	releases, err := action.NewHistory(c.conf).Run(project.Name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			c.log.Info("the release not found, installing it", "release", project.Name, "namespace", project.Name)
			install := action.NewInstall(c.conf)
			install.ReleaseName = project.Name
			install.Timeout = c.opts.Timeout
			install.UseReleaseName = true
			install.Labels = map[string]string{
				consts.ReleaseHashLabel: hash,
			}
			install.PostRenderer = post
			if _, err = install.RunWithContext(ctx, ch, values); err != nil {
				c.log.Error(err, "failed to install the release", "release", project.Name, "namespace", project.Name)
				return fmt.Errorf("failed to install the release: %w", err)
			}
			c.log.Info("the release installed", "release", project.Name, "namespace", project.Name)
			return nil
		}
		c.log.Error(err, "failed to retrieve history for the release", "release", project.Name, "namespace", project.Name)
		return fmt.Errorf("failed to retrieve history for the release: %w", err)
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	if releaseHash, ok := releases[0].Labels[consts.ReleaseHashLabel]; ok {
		if releaseHash == hash && releases[0].Info.Status == release.StatusDeployed {
			c.log.Info("the release is up to date", "release", project.Name, "namespace", project.Name)
			return nil
		}
	}

	if releases[0].Info.Status.IsPending() {
		if err = c.rollbackLatestRelease(releases); err != nil {
			c.log.Error(err, "failed to rollback the latest release", "release", project.Name, "namespace", project.Name)
			return fmt.Errorf("failed to rollback latest release: %w", err)
		}
	}

	upgrade := action.NewUpgrade(c.conf)
	upgrade.Install = true
	upgrade.MaxHistory = int(c.opts.HistoryMax)
	upgrade.Timeout = c.opts.Timeout
	upgrade.Labels = map[string]string{
		consts.ReleaseHashLabel: hash,
	}
	upgrade.PostRenderer = post

	if _, err = upgrade.RunWithContext(ctx, project.Name, ch, values); err != nil {
		c.log.Error(err, "failed to upgrade the release", "release", project.Name, "namespace", project.Name)
		return fmt.Errorf("failed to upgrade the release: %s", err)
	}

	c.log.Info("the release upgraded", "release", project.Name, "namespace", project.Name)
	return nil
}

func makeChart(templates map[string][]byte, releaseName string) (*chart.Chart, error) {
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
	return ch, nil
}

func buildValues(project *v1alpha2.Project, template *v1alpha1.ProjectTemplate) map[string]interface{} {
	structs.DefaultTagName = "yaml"
	preparedProject := struct {
		Name         string                 `json:"projectName" yaml:"projectName"`
		TemplateName string                 `json:"projectTemplateName" yaml:"projectTemplateName"`
		Parameters   map[string]interface{} `json:"parameters" yaml:"parameters"`
	}{
		Name:         project.Name,
		TemplateName: project.Spec.ProjectTemplateName,
		Parameters:   project.Spec.Parameters,
	}
	return map[string]interface{}{
		"projectTemplate": structs.Map(template.Spec),
		"project":         structs.Map(preparedProject),
	}
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

// Delete deletes resources
func (c *Client) Delete(_ context.Context, releaseName string) error {
	uninstall := action.NewUninstall(c.conf)
	uninstall.KeepHistory = false
	uninstall.IgnoreNotFound = true

	if _, err := uninstall.Run(releaseName); err != nil {
		c.log.Error(err, "failed to delete the release", "release", releaseName)
		return fmt.Errorf("failed to uninstall the %s release: %v", releaseName, err)
	}
	c.log.Info("the release deleted", "release", releaseName)
	return nil
}

// ValidateRender tests project render
func (c *Client) ValidateRender(project *v1alpha2.Project, template *v1alpha1.ProjectTemplate) error {
	ch, err := makeChart(c.templates, project.Name)
	if err != nil {
		c.log.Error(err, "failed to make chart", "release", project.Name, "namespace", project.Name)
		return err
	}

	values, err := chartutil.ToRenderValues(ch, buildValues(project, template), chartutil.ReleaseOptions{
		Name:      project.Name,
		Namespace: project.Name,
	}, nil)
	if err != nil {
		return err
	}

	rendered, err := engine.Render(ch, values)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)
	for _, file := range rendered {
		buf.WriteString(file)
	}

	_, err = newPostRenderer(project.Name, template.Name, c.log).Run(buf)
	return err
}
