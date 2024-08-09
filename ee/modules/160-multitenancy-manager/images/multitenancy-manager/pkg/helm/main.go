/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package helm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const helmDriver = "secret"

type Interface interface {
	Upgrade(ctx context.Context, opts *UpgradeOptions) error
	Delete(ctx context.Context, releaseName string) error
}

type client struct {
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

func WithDryRun() Option {
	return func(options *options) {
		options.DryRun = true
	}
}

// New initializes helm client with secret backend storage in `namespace` arg namespace.
// Possible options:
// WithHistoryMax - set maximum stored releases. Default: 3
// WithTimeout - timeout for helm upgrade/delete. Default: 15 seconds
// WithDryRun - enable dry run mode
func New(namespace, templatesPath string, log logr.Logger, opts ...Option) (Interface, error) {
	cli := client{
		opts: &options{
			HistoryMax: 3,
			Timeout:    time.Duration(15 * float64(time.Second)),
		},
		conf: &action.Configuration{},
		log:  log.WithName("helm"),
	}

	for _, opt := range opts {
		opt(cli.opts)
	}

	cli.log.Info("initializing action config")
	if err := cli.initActionConfig(namespace); err != nil {
		cli.log.Error(err, "failed to initialize action config")
		return nil, err
	}

	if err := cli.parseHelmTemplates(templatesPath); err != nil {
		cli.log.Error(err, "failed to parse helm templates")
		return nil, err
	}

	cli.log.Info("client initialized")
	return &cli, nil
}

func (c *client) parseHelmTemplates(templatesPath string) error {
	dir, err := os.ReadDir(templatesPath)
	if err != nil {
		return err
	}
	for _, file := range dir {
		if file.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(templatesPath, file.Name()))
		if err != nil {
			c.log.Error(err, "failed to read file", "file", file.Name())
			return err
		}
		c.log.Info("parsed file with project template", "file", file.Name())
		c.templates[file.Name()] = data
	}
	return nil
}

func (c *client) initActionConfig(namespace string) error {
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

	return c.conf.Init(kubeConfig, namespace, helmDriver, klog.Infof)
}

type UpgradeOptions struct {
	ReleaseName      string
	ReleaseNamespace string
	ProjectTemplate  string
	ProjectName      string
	Values           map[string]interface{}
}

// Upgrade upgrades resources
func (c *client) Upgrade(ctx context.Context, opts *UpgradeOptions) error {
	ch, err := c.makeChart(opts.ReleaseName)
	if err != nil {
		c.log.Error(err, "failed to make chart", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
		return err
	}

	hash := hashMD5(c.templates, opts.Values)
	postRenderer := newPostRenderer(opts.ProjectName, opts.ProjectTemplate, c.log)

	releases, err := action.NewHistory(c.conf).Run(opts.ReleaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			c.log.Info("release not found, installing it", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
			install := action.NewInstall(c.conf)
			install.Namespace = opts.ReleaseNamespace
			install.Timeout = c.opts.Timeout
			install.ReleaseName = opts.ReleaseName
			install.UseReleaseName = true
			install.Labels = map[string]string{
				"hashsum": hash,
			}
			install.PostRenderer = postRenderer
			if _, err = install.RunWithContext(ctx, ch, opts.Values); err != nil {
				c.log.Error(err, "failed to install release", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
				return err
			}
			c.log.Info("release installed", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
			return nil
		}
		c.log.Error(err, "failed to retrieve history for release", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
		return err
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	if releaseHash, ok := releases[0].Labels["hashsum"]; ok {
		if releaseHash == hash && releases[0].Info.Status == release.StatusDeployed {
			c.log.Info("release is up to date", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
			return nil
		}
	}

	if releases[0].Info.Status.IsPending() {
		if err = c.rollbackLatestRelease(releases); err != nil {
			c.log.Error(err, "failed to rollback latest release", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
			return err
		}
	}

	upgrade := action.NewUpgrade(c.conf)
	upgrade.Namespace = opts.ReleaseNamespace
	upgrade.Install = true
	upgrade.MaxHistory = int(c.opts.HistoryMax)
	upgrade.Timeout = c.opts.Timeout
	upgrade.Labels = map[string]string{
		"hashsum": hash,
	}
	upgrade.PostRenderer = postRenderer

	if _, err = upgrade.RunWithContext(ctx, opts.ReleaseName, ch, opts.Values); err != nil {
		c.log.Error(err, "failed to upgrade release", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
		return fmt.Errorf("failed to upgrade: %s", err)
	}

	c.log.Info("release upgraded", "release", opts.ReleaseName, "namespace", opts.ReleaseNamespace)
	return nil
}

func (c *client) makeChart(releaseName string) (*chart.Chart, error) {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    releaseName,
			Version: "0.0.1",
		},
	}
	for name, template := range c.templates {
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

func (c *client) rollbackLatestRelease(releases []*release.Release) error {
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
func (c *client) Delete(_ context.Context, releaseName string) error {
	uninstall := action.NewUninstall(c.conf)
	uninstall.KeepHistory = false
	uninstall.IgnoreNotFound = true

	if _, err := uninstall.Run(releaseName); err != nil {
		c.log.Error(err, "failed to delete release", "release", releaseName)
		return fmt.Errorf("failed to uninstall %s: %v", releaseName, err)
	}
	c.log.Info("release deleted", "release", releaseName)
	return nil
}
