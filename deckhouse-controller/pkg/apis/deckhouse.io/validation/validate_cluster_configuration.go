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

	"github.com/Masterminds/semver/v3"
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
	APIVersion              string       `json:"apiVersion"`
	Kind                    string       `json:"kind"`
	ClusterType             string       `json:"clusterType"`
	KubernetesVersion       string       `json:"kubernetesVersion"`
	DefaultCRI              string       `json:"defaultCRI"`
	PodSubnetNodeCIDRPrefix string       `json:"podSubnetNodeCIDRPrefix"`
	PodSubnetCIDR           string       `json:"podSubnetCIDR"`
	ServiceSubnetCIDR       string       `json:"serviceSubnetCIDR"`
	ClusterDomain           string       `json:"clusterDomain"`
	EncryptionAlgorithm     string       `json:"encryptionAlgorithm"`
	Cloud                   *cloudConfig `json:"cloud,omitempty"`
}

type cloudConfig struct {
	Provider string `json:"provider"`
	Prefix   string `json:"prefix,omitempty"`
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

func getKubernetesEndpointsCount(ctx context.Context, cli client.Client) (int, error) {
	endpoints := &v1.Endpoints{}
	err := cli.Get(ctx, client.ObjectKey{
		Namespace: "default",
		Name:      "kubernetes",
	}, endpoints)
	if err != nil {
		return 0, fmt.Errorf("failed to get kubernetes endpoints: %w", err)
	}

	count := 0
	for _, subset := range endpoints.Subsets {
		count += len(subset.Addresses)
	}
	return count, nil
}

func parseVersion(version string) (*semver.Version, error) {
	return semver.NewVersion(version)
}

func validateKubernetesVersionDowngrade(oldVersion, newVersion string, secret *v1.Secret) (*kwhvalidating.ValidatorResult, error) {
	if oldVersion == newVersion {
		return allowResult(nil)
	}

	if newVersion == "Automatic" {
		if oldVersion != "Automatic" {
			automaticVersionB64, exists := secret.Data["deckhouseDefaultKubernetesVersion"]
			if !exists {
				return allowResult(nil)
			}

			automaticVersion := string(automaticVersionB64)
			verAutomatic, err := parseVersion(automaticVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to parse automatic version: %w", err)
			}

			verOld, err := parseVersion(oldVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to parse old version: %w", err)
			}

			if verOld.GreaterThan(verAutomatic) {
				return rejectResult(fmt.Sprintf("can not set Automatic because it will downgrade kubernetes version. Automatic=%s oldKubernetesVersion=%s", automaticVersion, oldVersion))
			}
		}
		return allowResult(nil)
	}

	verOld, err := parseVersion(oldVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old version: %w", err)
	}

	verNew, err := parseVersion(newVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new version: %w", err)
	}

	// Check if downgrading more than 1 minor version
	if verOld.Major() > verNew.Major() || (verOld.Major() == verNew.Major() && verOld.Minor() > verNew.Minor()+1) {
		return rejectResult(fmt.Sprintf("can not downgrade kubernetes version more than 1 minor version. oldKubernetesVersion=%s newKubernetesVersion=%s", oldVersion, newVersion))
	}

	maxUsedVersionB64, exists := secret.Data["maxUsedControlPlaneKubernetesVersion"]
	if exists {
		maxUsedVersion := string(maxUsedVersionB64)
		verMax, err := parseVersion(maxUsedVersion)
		if err == nil {
			if verMax.Major() > verNew.Major() || (verMax.Major() == verNew.Major() && verMax.Minor() > verNew.Minor()+1) {
				return rejectResult(fmt.Sprintf("can not downgrade kubernetes version more than 1 minor version. maxUsedControlPlaneKubernetesVersion=%s newKubernetesVersion=%s", maxUsedVersion, newVersion))
			}
		}
	}

	return allowResult(nil)
}

func validateCRIChange(oldCRI, newCRI string, cli client.Client) (*kwhvalidating.ValidatorResult, error) {
	if oldCRI == newCRI {
		return allowResult(nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	endpointsCount, err := getKubernetesEndpointsCount(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints count: %w", err)
	}

	if endpointsCount < 3 {
		return allowResult([]string{"it is disruptive to change defaultCRI type for cluster with apiserver endpoints < 3"})
	}

	return allowResult(nil)
}

func validateUnsafeConfigChanges(oldConfig, newConfig *clusterConfig, unsafeMode bool) (*kwhvalidating.ValidatorResult, error) {
	if unsafeMode {
		return allowResult(nil)
	}

	if oldConfig.PodSubnetNodeCIDRPrefix != newConfig.PodSubnetNodeCIDRPrefix {
		return rejectResult("it is forbidden to change podSubnetNodeCIDRPrefix in a running cluster")
	}

	if oldConfig.PodSubnetCIDR != newConfig.PodSubnetCIDR {
		return rejectResult("it is forbidden to change podSubnetCIDR in a running cluster")
	}

	if oldConfig.ServiceSubnetCIDR != newConfig.ServiceSubnetCIDR {
		return rejectResult("it is forbidden to change serviceSubnetCIDR in a running cluster")
	}

	return allowResult(nil)
}

func validateClusterConfiguration(ctx context.Context, clusterConfiguration []byte) (*kwhvalidating.ValidatorResult, error) {
	_, err := config.ParseConfigFromData(ctx, string(clusterConfiguration), config.DummyPreparatorProvider(), config.ValidateOptionOmitDocInError(true))
	if err != nil {
		result, _ := rejectResult(err.Error())
		return result, nil
	}

	result, _ := allowResult(nil)
	return result, nil
}

func clusterConfigurationHandler(mm moduleManager, cli client.Client, _ *config.SchemaStore) http.Handler {
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

		clusterConfigurationValidator := kwhvalidating.ValidatorFunc(func(ctx context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateClusterConfiguration(ctx, clusterConfigurationRaw)
		})

		clusterConf := new(clusterConfig)
		if err := yaml.Unmarshal(clusterConfigurationRaw, clusterConf); err != nil {
			log.Debug("failed to unmarshal cluster configuration", log.Err(err))
			return nil, fmt.Errorf("unmarshal cluster configuration: %w", err)
		}

		k8sVersionValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			if clusterConf.KubernetesVersion == "Automatic" {
				return allowResult(nil)
			}
			return validateKubernetesVersion(clusterConf.KubernetesVersion, mm)
		})

		criValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
			return validateDefaultCRI(clusterConf.DefaultCRI, cli)
		})

		validators := []kwhvalidating.Validator{clusterConfigurationValidator, k8sVersionValidator, criValidator}

		if ar.Operation == model.OperationUpdate && ar.OldObjectRaw != nil {
			oldSecret := &v1.Secret{}
			if err := yaml.Unmarshal(ar.OldObjectRaw, oldSecret); err == nil {
				if oldClusterConfigurationRaw, ok := oldSecret.Data["cluster-configuration.yaml"]; ok {
					oldClusterConf := new(clusterConfig)
					if err := yaml.Unmarshal(oldClusterConfigurationRaw, oldClusterConf); err == nil {
						unsafeMode := false
						if annotations := secret.GetAnnotations(); annotations != nil {
							if annotations["deckhouse.io/allow-unsafe"] != "" && annotations["deckhouse.io/allow-unsafe"] != "null" {
								unsafeMode = true
							}
						}

						unsafeValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							return validateUnsafeConfigChanges(oldClusterConf, clusterConf, unsafeMode)
						})

						k8sDowngradeValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							return validateKubernetesVersionDowngrade(oldClusterConf.KubernetesVersion, clusterConf.KubernetesVersion, secret)
						})

						criChangeValidator := kwhvalidating.ValidatorFunc(func(_ context.Context, _ *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
							oldCRI := oldClusterConf.DefaultCRI
							if oldCRI == "" {
								oldCRI = "Containerd"
							}
							newCRI := clusterConf.DefaultCRI
							if newCRI == "" {
								newCRI = "Containerd"
							}
							return validateCRIChange(oldCRI, newCRI, cli)
						})

						validators = append(validators, unsafeValidator, k8sDowngradeValidator, criChangeValidator)
					}
				}
			}
		}

		chain := kwhvalidating.NewChain(nil, validators...)
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
