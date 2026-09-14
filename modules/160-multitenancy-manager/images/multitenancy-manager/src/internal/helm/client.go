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
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	helmDriver = "secret"

	ResourceAnnotationReleaseName      = "meta.helm.sh/release-name"
	ResourceAnnotationReleaseNamespace = "meta.helm.sh/release-namespace"

	ResourceLabelManagedBy = "app.kubernetes.io/managed-by"
)

// Client drives the Helm release of every project. Helm is the apply engine only -- install,
// upgrade, prune of what left the render, history, rollback of a pending release -- and templates
// nothing: the objects are rendered natively from the structured ProjectTemplate
// (controller/internal/render) and handed to the post-renderer as they are, so no user data ever
// passes through the Helm template engine.
type Client struct {
	conf   *action.Configuration
	opts   *options
	logger logr.Logger
}

type options struct {
	HistoryMax int32
	Timeout    time.Duration
}

// New initializes helm client with secret backend storage in `namespace` arg namespace.
func New(namespace string, logger logr.Logger) (*Client, error) {
	cli := &Client{
		opts: &options{
			HistoryMax: 3,
			Timeout:    15 * time.Second,
		},
		conf: &action.Configuration{
			Capabilities: chartutil.DefaultCapabilities,
		},
		logger: logger.WithName("helm"),
	}

	cli.logger.Info("initializing action config")
	if err := cli.initActionConfig(namespace); err != nil {
		return nil, fmt.Errorf("initialize action config: %w", err)
	}

	cli.logger.Info("client initialized")
	return cli, nil
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

// UpgradeManifests installs/upgrades the project release from manifests rendered natively from the
// structured ProjectTemplate. The chart is a no-op (empty) template: the concrete objects are
// supplied to the post-renderer, so no user data passes through the Helm template engine while Helm
// still drives the release lifecycle (install/upgrade/prune/history). The release hash is taken over
// the rendered manifests, so a structural or parameter change re-applies the release and an
// unchanged render is a no-op.
func (c *Client) UpgradeManifests(ctx context.Context, project *v1alpha3.Project, manifests string) error {
	versions, err := c.discoverAPI()
	if err != nil {
		return fmt.Errorf("discover api: %w", err)
	}

	// The Helm release name is derived from the project name: it equals the project name when it fits
	// Helm's 53-char limit and is deterministically shortened otherwise (see ReleaseName). The project
	// namespace stays the raw project name.
	rel := ReleaseName(project.Name)
	ch := buildEmptyChart(project.Name)
	hash := hashString(manifests)

	// action.History exposes no context or timeout; the lookup is a single, bounded API read.
	releases, err := action.NewHistory(c.conf).Run(rel)
	if err != nil {
		if !errors.Is(err, driver.ErrReleaseNotFound) {
			return fmt.Errorf("retrieve history for the release: %w", err)
		}

		c.logger.Info("the release not found, install it", "release", rel, "namespace", project.Name)
		post := newPostRenderer(project, versions, c.logger)
		post.manifests = manifests
		install := action.NewInstall(c.conf)
		install.ReleaseName = rel
		install.Timeout = c.opts.Timeout
		install.UseReleaseName = true
		install.Labels = map[string]string{
			v1alpha3.ReleaseLabelHashsum: hash,
		}
		install.PostRenderer = post
		if _, err = install.RunWithContext(ctx, ch, map[string]any{}); err != nil {
			return fmt.Errorf("install the release: %w", err)
		}
		c.logger.Info("the release installed", "release", rel, "namespace", project.Name)
		return nil
	}

	releaseutil.Reverse(releases, releaseutil.SortByRevision)
	if releaseHash, ok := releases[0].Labels[v1alpha3.ReleaseLabelHashsum]; ok {
		if releaseHash == hash && releases[0].Info.Status == release.StatusDeployed {
			c.logger.Info("the release is up to date", "release", rel, "namespace", project.Name)
			return nil
		}
	}

	if releases[0].Info.Status.IsPending() {
		if err = c.rollbackLatestRelease(releases); err != nil {
			return fmt.Errorf("rollback latest release: %w", err)
		}
	}

	post := newPostRenderer(project, versions, c.logger)
	post.manifests = manifests
	upgrade := action.NewUpgrade(c.conf)
	upgrade.Install = true
	upgrade.MaxHistory = int(c.opts.HistoryMax)
	upgrade.Timeout = c.opts.Timeout
	upgrade.Labels = map[string]string{
		v1alpha3.ReleaseLabelHashsum: hash,
	}
	upgrade.PostRenderer = post

	if _, err = upgrade.RunWithContext(ctx, rel, ch, map[string]any{}); err != nil {
		return fmt.Errorf("upgrade the release: %w", err)
	}

	c.logger.Info("the release upgraded", "release", rel, "namespace", project.Name)
	return nil
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

// ReleaseName maps a project name to its Helm release name. A name within Helm's limit is used
// verbatim so existing releases keep their names (no migration); a longer name is deterministically
// shortened to a collision-resistant form (a truncated prefix plus an 8-hex md5 suffix of the full
// name) that stays a valid, unique release name. The project namespace remains the raw project name.
func ReleaseName(project string) string {
	if len(project) <= helmReleaseNameMaxLen {
		return project
	}
	const suffixLen = 9 // '-' + 8 hex characters
	prefix := strings.TrimRight(project[:helmReleaseNameMaxLen-suffixLen], "-")
	return prefix + "-" + hashString(project)[:8]
}

// buildEmptyChart builds a chart whose only template renders to nothing. The real objects are
// supplied to the post-renderer, so the chart exists only to drive Helm's release machinery and must
// not template any user data.
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
	rel := ReleaseName(projectName)
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
