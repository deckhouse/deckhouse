/*
Copyright 2021 Flant JSC

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
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const configPath = `controlPlaneManager.internal.kubeSchedulerExtenders`

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "kube_scheduler_extenders",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "KubeSchedulerWebhookConfiguration",
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			FilterFunc:                   extendersFilter,
		},
	},
}, handleExtenders)

func extendersFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var extenderCR KubeSchedulerWebhookConfiguration

	err := sdk.FromUnstructured(obj, &extenderCR)
	return extenderCR.Webhooks, err
}
func handleExtenders(_ context.Context, input *go_hook.HookInput) error {
	type extenderConfig struct {
		URLPrefix string `yaml:"urlPrefix" json:"urlPrefix"`
		Weight    int    `yaml:"weight" json:"weight"`
		Timeout   int    `yaml:"timeout" json:"timeout"`
		Ignorable bool   `yaml:"ignorable" json:"ignorable"`
		CAData    string `yaml:"caData" json:"caData"`
	}
	extenders := make([]extenderConfig, 0)

	var clusterDomain = input.Values.Get("global.discovery.clusterDomain").String()
	var kubernetesCABase64 = base64.StdEncoding.EncodeToString([]byte(input.Values.Get("global.discovery.kubernetesCA").String()))
	for snapshot, err := range sdkobjectpatch.SnapshotIter[[]KubeSchedulerWebhook](input.Snapshots.Get("kube_scheduler_extenders")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshot: %w", err)
		}

		for _, config := range snapshot {
			err = verifyCAChain(config.ClientConfig.CABundle)
			if err != nil {
				input.Logger.Warn("failed to verify CA chain, use default kubernetes CA", log.Err(err))
				config.ClientConfig.CABundle = kubernetesCABase64
			}

			urlPrefix, err := url.JoinPath(fmt.Sprintf("https://%s.%s.svc.%s:%d", config.ClientConfig.Service.Name, config.ClientConfig.Service.Namespace, clusterDomain, config.ClientConfig.Service.Port), config.ClientConfig.Service.Path)
			if err != nil {
				return err
			}
			newExtender := extenderConfig{
				URLPrefix: urlPrefix,
				Weight:    config.Weight,
				Timeout:   config.TimeoutSeconds,
				Ignorable: config.FailurePolicy == "Ignore",
				CAData:    config.ClientConfig.CABundle,
			}
			extenders = append(extenders, newExtender)
		}
	}
	input.Values.Set(configPath, extenders)
	return nil
}

func verifyCAChain(caBase64 string) error {
	caData, err := base64.StdEncoding.DecodeString(caBase64)
	if err != nil {
		return err
	}
	_, err = certificate.ParseCertificate(string(caData))
	return err
}

type KubeSchedulerWebhookConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Webhooks          []KubeSchedulerWebhook `json:"webhooks" yaml:"webhooks"`
}

type KubeSchedulerWebhook struct {
	Weight         int                              `json:"weight" yaml:"weight"`
	FailurePolicy  string                           `json:"failurePolicy" yaml:"failurePolicy"`
	ClientConfig   KubeSchedulerWebhookClientConfig `json:"clientConfig" yaml:"clientConfig"`
	TimeoutSeconds int                              `json:"timeoutSeconds" yaml:"timeoutSeconds"`
}

type KubeSchedulerWebhookClientConfig struct {
	Service  KubeSchedulerWebhookService `json:"service" yaml:"service"`
	CABundle string                      `json:"caBundle" yaml:"caBundle"`
}

type KubeSchedulerWebhookService struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Port      int    `json:"port" yaml:"port"`
	Path      string `json:"path" yaml:"path"`
}
