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

package validation

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	containerdV2UnsupportedLabel        = "node.deckhouse.io/containerd-v2-unsupported"
	customContainerdConfigLabelSelector = "node.deckhouse.io/containerd-config=custom"
)

type clusterConfig struct {
	KubernetesVersion string `json:"kubernetesVersion"`
	DefaultCRI        string `json:"defaultCRI"`
}

func validateKubernetesVersion(version string, mm moduleManager) (*kwhvalidating.ValidatorResult, error) {
	if version == "Automatic" {
		version = hooks.DefaultKubernetesVersion
	}

	if moduleName, err := kubernetesversion.Instance().ValidateBaseVersion(version); err != nil {
		log.Debug("failed to validate base version", log.Err(err))
		if moduleName == "" {
			return rejectResult(err.Error())
		}
		if mm.IsModuleEnabled(moduleName) {
			log.Debug("module has unsatisfied requirements", slog.String("name", moduleName))
			return rejectResult(err.Error())
		}
	}

	return allowResult(nil)
}

func checkCntrdV2Support(ctx context.Context, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	unsupportedSelector, err := labels.Parse(containerdV2UnsupportedLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector for unsupported nodes: %w", err)
	}

	unsupportedNodes := &v1.NodeList{}
	if err := cli.List(ctx, unsupportedNodes, &client.ListOptions{LabelSelector: unsupportedSelector}); err != nil {
		return nil, fmt.Errorf("failed to list nodes with label %q: %w", containerdV2UnsupportedLabel, err)
	}

	if len(unsupportedNodes.Items) > 0 {
		return rejectResult("Cluster has nodes that don't support ContainerdV2")
	}

	customConfigSelector, err := labels.Parse(customContainerdConfigLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector for custom containerd config: %w", err)
	}

	customConfigNodes := &v1.NodeList{}
	if err := cli.List(ctx, customConfigNodes, &client.ListOptions{LabelSelector: customConfigSelector}); err != nil {
		return nil, fmt.Errorf("failed to list nodes with label %q: %w", customContainerdConfigLabelSelector, err)
	}

	if len(customConfigNodes.Items) > 0 {
		return rejectResult("Cluster has nodes with a custom containerd config, which is incompatible with ContainerdV2")
	}

	return allowResult(nil)
}

func validateDefaultCRI(defaultCRI string, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch defaultCRI {
	case "Containerd":
		return allowResult(nil)
	case "ContainerdV2":
		return checkCntrdV2Support(ctx, cli)
	case "NotManaged":
		return allowResult(nil)
	default:
		return rejectResult(fmt.Sprintf("Unsupported CRI: %s", defaultCRI))
	}
}

func validateClusterConfiguration(schemaStore *config.SchemaStore, clusterConfiguration []byte) (*kwhvalidating.ValidatorResult, error) {
	_, err := schemaStore.Validate(&clusterConfiguration, config.ValidateOptionOmitDocInError(true))
	if err != nil {
		return rejectResult(err.Error())
	}

	return allowResult(nil)
}

func clusterConfigurationHandler(mm moduleManager, cli client.Client, schemaStore *config.SchemaStore) http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(ctx context.Context, ar *model.AdmissionReview, obj metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		if ar.Operation == model.OperationDelete {
			return rejectResult("It is forbidden to delete secret d8-cluster-configuration")
		}

		secret, ok := obj.(*v1.Secret)
		if !ok {
			log.Debug("unexpected type", log.Type("expected", v1.Secret{}), log.Type("got", obj))
			return nil, fmt.Errorf("expect Secret as unstructured, got %T", obj)
		}

		clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
		if !ok {
			log.Debug("no cluster-configuration found in secret", slog.String("namespace", obj.GetNamespace()), slog.String("name", obj.GetName()))
			return nil, fmt.Errorf("expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
		}

		clusterConfigurationValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateClusterConfiguration(schemaStore, clusterConfigurationRaw)
		})

		clusterConf := new(clusterConfig)
		if err := yaml.Unmarshal(clusterConfigurationRaw, clusterConf); err != nil {
			log.Debug("failed to unmarshal cluster configuration", log.Err(err))
			return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
		}

		k8sVersionValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateKubernetesVersion(clusterConf.KubernetesVersion, mm)
		})

		criValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateDefaultCRI(clusterConf.DefaultCRI, cli)
		})

		chain := kwhvalidating.NewChain(nil, clusterConfigurationValidator, k8sVersionValidator, criValidator)
		return chain.Validate(ctx, ar, obj)
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "cluster-configuration-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1.Secret{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
