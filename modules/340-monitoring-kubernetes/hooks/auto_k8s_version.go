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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/kube-client/manifest/releaseutil"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	AutoK8sVersion = "autoK8sVersion"
	AutoK8sReason  = "autoK8sReason"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes/auto_k8s_version",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "auto_k8s_version",
			Crontab: "0 * * * *", // every hour
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "kubernetesVersion",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        applyClusterConfigurationYamlFilter,
		},
	},
}, dependency.WithExternalDependencies(clusterConfiguration))

type ClusterConfigurationYaml struct {
	Content []byte
}

func applyClusterConfigurationYamlFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	var metaConfig *config.MetaConfig
	metaConfig, err = config.ParseConfigFromData(string(ccYaml))
	if err != nil {
		return nil, err
	}

	kubernetesVersion, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
	if err != nil {
		return nil, err
	}

	return kubernetesVersion, err
}

func clusterConfiguration(input *go_hook.HookInput, dc dependency.Container) error {
	kubernetesVersion, ok := input.Snapshots["kubernetesVersion"]
	if ok && len(kubernetesVersion) > 0 && kubernetesVersion[0].(string) == "Automatic" {
		var (
			unsupportVersion k8sUnsupportedVersion
			wg               sync.WaitGroup
		)

		// create buffered channel == objectBatchSize
		// this give as ability to handle in memory only objectBatchSize * 2 amount of helm releases
		// because this counter also used as a limit to apiserver
		// we have `objectBatchSize` (10) objects in channel and max `objectBatchSize` (10) objects in goroutine waiting for channel
		releasesC := make(chan *release, objectBatchSize)
		doneC := make(chan bool)

		go unsupportVersion.runReleaseVerify(input, releasesC, doneC)

		ctx := context.Background()
		client, err := dc.GetK8sClient()
		if err != nil {
			return err
		}

		wg.Add(2)
		go func() {
			defer wg.Done()
			var err error
			_, err = getHelm3Releases(ctx, client, releasesC)
			if err != nil {
				input.LogEntry.Error(err)
				return
			}
		}()

		go func() {
			defer wg.Done()
			var err error
			_, err = getHelm2Releases(ctx, client, releasesC)
			if err != nil {
				input.LogEntry.Error(err)
				return
			}
		}()

		wg.Wait()
		close(releasesC)
		<-doneC

		k8sVersion, reason := unsupportVersion.get()
		if k8sVersion != "" {
			requirements.SaveValue(AutoK8sVersion, k8sVersion)
			requirements.SaveValue(AutoK8sReason, reason)
		}

		return nil
	}

	// unset unavailabel k8s vesion
	requirements.RemoveValue(AutoK8sVersion)
	requirements.RemoveValue(AutoK8sReason)

	return nil
}

func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	b, err := message.MarshalJSON()
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}

type k8sUnsupportedVersion struct {
	k8sVersion *semver.Version
	reasons    map[string]struct{}
}

func (uv *k8sUnsupportedVersion) runReleaseVerify(input *go_hook.HookInput, releasesC <-chan *release, doneC chan<- bool) {
	defer func() {
		doneC <- true
	}()
	for rel := range releasesC {
		for _, manifestData := range releaseutil.SplitManifests(rel.Manifest) {
			resource := new(manifest)
			err := yaml.Unmarshal([]byte(manifestData), &resource)
			if err != nil {
				input.LogEntry.Errorf("manifest (%s/%s) read error: %s", rel.Namespace, rel.Name, err)
				continue
			}

			if resource == nil {
				continue
			}

			reason := fmt.Sprintf("%s: %s", resource.APIVersion, resource.Kind)
			for version, store := range helmStorage {
				if store.isUnsupportedByAPIAndKind(resource.APIVersion, resource.Kind) {
					k8sVersion := semver.MustParse(version)
					switch {
					case uv.k8sVersion == nil || uv.k8sVersion.GreaterThan(k8sVersion):
						uv.k8sVersion = k8sVersion
						uv.reasons = map[string]struct{}{
							reason: {},
						}
					case uv.k8sVersion != nil && uv.k8sVersion.Equal(k8sVersion):
						uv.reasons[reason] = struct{}{}
					}
				}
			}
		}
	}
}

func (uv *k8sUnsupportedVersion) get() (k8sVersion, reasons string) {
	keys := make([]string, 0, len(uv.reasons))
	for key := range uv.reasons {
		keys = append(keys, key)
	}
	if uv.k8sVersion != nil {
		return uv.k8sVersion.String(), strings.Join(keys, ", ")
	}

	return "", ""
}
