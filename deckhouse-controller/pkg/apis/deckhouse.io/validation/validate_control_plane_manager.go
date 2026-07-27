/*
Copyright 2026 Flant JSC

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
	"slices"

	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	clusterKubernetesConfigMapName = "d8-cluster-kubernetes"
	clusterConfigurationSecretName = "d8-cluster-configuration"
	clusterConfigurationSecretNS   = "kube-system"
	clusterConfigurationDataKey    = "cluster-configuration.yaml"
	clusterKubernetesStatusDataKey = "status"
)

// clusterKubernetesStatus is the subset of ConfigMap d8-cluster-kubernetes data.status
// needed for admission. Written by update-observer.
type clusterKubernetesStatus struct {
	AvailableVersions []string `yaml:"availableVersions"`
}

// validateControlPlaneManagerKubernetesVersion guards ModuleConfig control-plane-manager's
// kubernetesVersion against membership in status.availableVersions from ConfigMap
// kube-system/d8-cluster-kubernetes (the set update-observer publishes as Supported[maxUsed-1:]).
// Explicit versions are also checked against module compatibility (validateKubernetesVersion).
//
// Automatic / unset values are not subject to membership. Removing a prior ModuleConfig pin
// (or deleting the ModuleConfig) resolves the future effective version via ClusterConfiguration
// and applies the same membership check, so a stale CC pin cannot silently become the target.
//
// Unchanged kubernetesVersion skips the check so edits to unrelated settings are not blocked by an
// orphaned pin that fell outside availableVersions after the ConfigMap appeared or Supported shrank.
//
// Fail-open on missing/empty ConfigMap status, empty availableVersions, or read errors: the
// ModuleConfig webhook runs with failurePolicy: Fail, and a transient outage must not lock out edits.
func (v *moduleConfigValidator) validateControlPlaneManagerKubernetesVersion(
	ctx context.Context, newSettings, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	newVersion := settingsKubernetesVersion(newSettings)
	oldVersion := settingsKubernetesVersion(oldSettings)

	if newVersion == oldVersion {
		return nil, nil
	}

	var (
		effective    string
		fromFallback bool
	)
	switch {
	case isPinnedKubernetesVersion(newVersion):
		effective = newVersion
	case isPinnedKubernetesVersion(oldVersion):
		// Clearing or deleting an override: effective falls back to CC, then Automatic.
		ccVersion, ok := v.readRawClusterConfigurationVersion(ctx)
		if !ok {
			return nil, nil
		}
		if !isPinnedKubernetesVersion(ccVersion) {
			return nil, nil
		}
		effective = ccVersion
		fromFallback = true
	default:
		// Automatic / unset without clearing a prior pin (HV-06, HV-07).
		return nil, nil
	}

	// validateKubernetesVersion is shared with the ClusterConfiguration webhook, where it is
	// chained as a kwhvalidating.Validator: always non-nil result, Valid=true on success. That
	// differs from validateCommon's "nil,nil means allow" convention, so results are translated.
	res, err := validateKubernetesVersion(effective, v.moduleManager)
	if err != nil {
		return nil, err
	}
	if res != nil && !res.Valid {
		return res, nil
	}

	available, ok := v.readAvailableKubernetesVersions(ctx)
	if !ok || len(available) == 0 {
		return nil, nil
	}

	if !slices.Contains(available, effective) {
		if fromFallback {
			return rejectResult(fmt.Sprintf(
				"clearing or deleting the ModuleConfig kubernetesVersion override would fall back to "+
					"ClusterConfiguration.kubernetesVersion %q, which is not in the cluster's availableVersions %v; "+
					"downgrading more than one minor below the highest version the cluster has ever run is forbidden",
				effective, available,
			))
		}
		return rejectResult(fmt.Sprintf(
			"kubernetesVersion %q is not in the cluster's availableVersions %v; "+
				"downgrading more than one minor below the highest version the cluster has ever run is forbidden",
			effective, available,
		))
	}
	return nil, nil
}

func settingsKubernetesVersion(settings map[string]interface{}) string {
	if settings == nil {
		return ""
	}
	version, _ := settings["kubernetesVersion"].(string)
	return version
}

func isPinnedKubernetesVersion(version string) bool {
	return version != "" && version != "Automatic"
}

// rawModuleConfigSettings returns spec.settings as stored on the object, without schema
// conversion or validateCR. Used for the kubernetesVersion clear/DELETE guard so a conversion
// failure on an unrelated field cannot hide an existing pin.
func rawModuleConfigSettings(cfg *v1alpha1.ModuleConfig) map[string]interface{} {
	if cfg == nil || cfg.Spec.Settings == nil {
		return nil
	}
	m := cfg.Spec.Settings.GetMap()
	if len(m) == 0 {
		return nil
	}
	return m
}

// readAvailableKubernetesVersions returns status.availableVersions from
// kube-system/d8-cluster-kubernetes. ok=false means fail-open (missing/empty/error).
func (v *moduleConfigValidator) readAvailableKubernetesVersions(ctx context.Context) (versions []string, ok bool) {
	cm := &v1.ConfigMap{}
	if err := v.client.Get(ctx, client.ObjectKey{
		Name:      clusterKubernetesConfigMapName,
		Namespace: clusterConfigurationSecretNS,
	}, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("skipping the kubernetesVersion availableVersions guard: cannot read the d8-cluster-kubernetes ConfigMap", log.Err(err))
		}
		return nil, false
	}

	raw, found := cm.Data[clusterKubernetesStatusDataKey]
	if !found || raw == "" {
		return nil, false
	}

	status := new(clusterKubernetesStatus)
	if err := yaml.Unmarshal([]byte(raw), status); err != nil {
		log.Warn("skipping the kubernetesVersion availableVersions guard: cannot parse d8-cluster-kubernetes status", log.Err(err))
		return nil, false
	}
	if len(status.AvailableVersions) == 0 {
		return nil, false
	}
	return status.AvailableVersions, true
}

// readRawClusterConfigurationVersion returns the literal kubernetesVersion from the
// d8-cluster-configuration Secret. Used only when resolving fallback after clearing an MC pin.
// ok=false means fail-open.
func (v *moduleConfigValidator) readRawClusterConfigurationVersion(ctx context.Context) (version string, ok bool) {
	secret := &v1.Secret{}
	if err := v.client.Get(ctx, client.ObjectKey{
		Name:      clusterConfigurationSecretName,
		Namespace: clusterConfigurationSecretNS,
	}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("skipping the kubernetesVersion fallback guard: cannot read the d8-cluster-configuration secret", log.Err(err))
		}
		return "", false
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data[clusterConfigurationDataKey], cc); err != nil {
		return "", false
	}
	return cc.KubernetesVersion, true
}
