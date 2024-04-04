/*
Copyright 2023 Flant JSC

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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"maps"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const helmDriver = "secret"

type Client interface {
	Upgrade(releaseName, releaseNamespace string, templates, values map[string]interface{}, debug bool, pr ...postrender.PostRenderer) error
	Delete(releaseName string) error
}

type helmClient struct {
	actionConfig *action.Configuration
	options      helmOptions
}

// NewClient initializes helm client with secret backend storage in `namespace` arg namespace.
// Possible options:
// WithHistoryMax - set maximum stored releases. Default: 3
// WithTimeout - timeout for helm upgrade/delete. Default: 15 seconds
func NewClient(namespace string, options ...Option) (Client, error) {
	opts := &helmOptions{
		HistoryMax: 3,
		Timeout:    time.Duration(15 * float64(time.Second)),
	}

	for _, opt := range options {
		opt(opts)
	}

	conf, err := getActionConfig(namespace)
	if err != nil {
		return nil, err
	}

	client := helmClient{
		actionConfig: conf,
		options:      *opts,
	}

	return &client, nil
}

type helmOptions struct {
	HistoryMax int32
	Timeout    time.Duration
}

type Option func(options *helmOptions)

func WithHistoryMax(historyMax int32) Option {
	return func(options *helmOptions) {
		options.HistoryMax = historyMax
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(options *helmOptions) {
		options.Timeout = timeout
	}
}

func (client *helmClient) Upgrade(releaseName, releaseNamespace string, templates, values map[string]interface{}, _ bool, pr ...postrender.PostRenderer) error {
	ch := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    releaseName,
			Version: "0.0.1",
		},
	}

	for name, template := range templates {
		data, ok := template.([]byte)
		if !ok {
			return fmt.Errorf("invalid template. Template name: %v", name)
		}

		chartFile := chart.File{
			Name: name,
			Data: data,
		}

		ch.Templates = append(ch.Templates, &chartFile)
		fmt.Println("#! TEMPLATES", ch.Templates)
	}
	hashsum := getMD5Hash(templates, values)

	upgradeObject := action.NewUpgrade(client.actionConfig)
	upgradeObject.Namespace = releaseNamespace
	upgradeObject.Install = true
	upgradeObject.MaxHistory = int(client.options.HistoryMax)
	upgradeObject.Timeout = client.options.Timeout
	upgradeObject.Labels = map[string]string{
		"hashsum": hashsum,
	}
	if len(pr) > 0 {
		upgradeObject.PostRenderer = pr[0]
	}

	releases, err := action.NewHistory(client.actionConfig).Run(releaseName)
	if err == driver.ErrReleaseNotFound {
		installObject := action.NewInstall(client.actionConfig)
		installObject.Namespace = releaseNamespace
		installObject.Timeout = client.options.Timeout
		installObject.ReleaseName = releaseName
		installObject.UseReleaseName = true
		installObject.Labels = map[string]string{
			"hashsum": hashsum,
		}
		if len(pr) > 0 {
			installObject.PostRenderer = pr[0]
		}

		_, err = installObject.Run(ch, values)
		return err
	}

	if len(releases) > 0 {
		releaseutil.Reverse(releases, releaseutil.SortByRevision)
		latestRelease := releases[0]
		val, ok := latestRelease.Labels["hashsum"]
		if ok {
			if val == hashsum && latestRelease.Info.Status == release.StatusDeployed {
				klog.Info("the hashes matched")
				return nil
			}
		}

		if latestRelease.Info.Status.IsPending() {
			client.rollbackLatestRelease(releases)
		}
	}

	_, err = upgradeObject.Run(releaseName, ch, values)
	if err != nil {
		return fmt.Errorf("helm upgrade failed: %s", err)
	}

	return nil
}

func (client *helmClient) Delete(releaseName string) error {
	uninstallObject := action.NewUninstall(client.actionConfig)
	uninstallObject.KeepHistory = false
	uninstallObject.IgnoreNotFound = true

	if _, err := uninstallObject.Run(releaseName); err != nil {
		return fmt.Errorf("helm uninstall %s invocation error: %v", releaseName, err)
	}

	return nil
}

func (client *helmClient) rollbackLatestRelease(releases []*release.Release) {
	latestRelease := releases[0]

	if latestRelease.Version == 1 || client.options.HistoryMax == 1 || len(releases) == 1 {
		uninstallObject := action.NewUninstall(client.actionConfig)
		uninstallObject.KeepHistory = false
		_, err := uninstallObject.Run(latestRelease.Name)
		if err != nil {
			return
		}
	} else {
		previousVersion := latestRelease.Version - 1
		for i := 1; i < len(releases); i++ {
			if !releases[i].Info.Status.IsPending() {
				previousVersion = releases[i].Version
				break
			}
		}
		rollbackObject := action.NewRollback(client.actionConfig)
		rollbackObject.Version = previousVersion
		rollbackObject.CleanupOnFail = true
		err := rollbackObject.Run(latestRelease.Name)
		if err != nil {
			return
		}
	}
}

func getActionConfig(namespace string) (*action.Configuration, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace must be specified")
	}
	actionConfig := new(action.Configuration)
	var kubeConfig *genericclioptions.ConfigFlags
	// Create the rest config instance with ServiceAccount values loaded in them
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig = genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	kubeConfig.Namespace = &namespace
	if err := actionConfig.Init(kubeConfig, namespace, helmDriver, klog.Infof); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

func getMD5Hash(templates, values map[string]interface{}) string {
	sum := make(map[string]interface{})
	maps.Copy(sum, templates)
	for k, v := range values {
		sum[k] = v
	}

	hash := md5.New()
	hashObject(sum, &hash)
	res := hash.Sum(nil)

	return hex.EncodeToString(res)
}
