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
	"log/slog"
	"net/http"
	"slices"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	"github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	clusterKubernetesConfigMapName = "d8-cluster-kubernetes"
	clusterConfigurationSecretName = "d8-cluster-configuration"
	kubeSystemNamespace            = "kube-system"

	// ClusterConfiguration's "track Deckhouse default" sentinel; ModuleConfig accepts only Default.
	automaticKubernetesVersion       = "Automatic"
	defaultKubernetesVersionSentinel = "Default"
)

// The tags are `json` because sigs.k8s.io/yaml goes through encoding/json.
type clusterKubernetesStatus struct {
	AvailableVersions []string `json:"availableVersions"`
	// What "Default" resolves to for the running build.
	AutomaticVersion string `json:"automaticVersion"`
}

type clusterKubernetesSpec struct {
	// The highest minor the cluster has ever converged onto; see update-observer's controller.Spec.
	MaxUsedVersion string `json:"maxUsedKubernetesVersion"`
}

// Guards kubernetesVersion against status.availableVersions of d8-cluster-kubernetes, plus module
// compatibility and the maxUsed floor. "Default" and an unchanged value are exempt from membership.
// Fail-open on anything unreadable: this webhook runs with failurePolicy: Fail.
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
	case isModuleConfigTrackDefault(newVersion):
		return nil, nil
	case newVersion != "":
		effective = newVersion
	case oldVersion != "":
		// Clearing the setting: effective falls back to CC, then the Deckhouse default.
		ccVersion, ok := v.readRawClusterConfigurationVersion(ctx)
		if !ok {
			return nil, nil
		}
		if !isClusterConfigurationPinned(ccVersion) {
			return nil, nil
		}
		effective = ccVersion
		fromFallback = true
	default:
		// Never set on either side (HV-06, HV-07).
		return nil, nil
	}

	// Shared with the ClusterConfiguration webhook, which uses "non-nil result, Valid=true" rather
	// than validateCommon's "nil,nil means allow" — hence the translation below.
	res, err := validateKubernetesVersion(effective, v.moduleManager)
	if err != nil {
		return nil, err
	}
	if res != nil && !res.Valid {
		return res, nil
	}

	// One read for both remaining guards: judging a version against two snapshots of the same
	// ConfigMap would look exactly like a bug.
	facts := v.readKubernetesVersionFacts(ctx)

	// Unconditional: membership alone misses deep downgrades before status is published.
	if res, err := rejectKubernetesVersionBelowMaxUsed(effective, fromFallback, facts); res != nil || err != nil {
		return res, err
	}

	available := facts.AvailableVersions
	if len(available) == 0 {
		return nil, nil
	}

	if !slices.Contains(available, effective) {
		// The list is bounded on both ends, so a miss is not necessarily a downgrade — printing it in
		// full shows the operator which side the value fell on.
		subject := fmt.Sprintf("kubernetesVersion %q", effective)
		if fromFallback {
			subject = fmt.Sprintf(
				"clearing or deleting the ModuleConfig kubernetesVersion override would fall back to "+
					"ClusterConfiguration.kubernetesVersion %q, which", effective)
		}

		msg := fmt.Sprintf("%s is not in the cluster's availableVersions %v", subject, available)
		return rejectResult(msg)
	}
	return nil, nil
}

// Deliberately not validateKubernetesVersionDowngrade: that one forbids handing control back to
// Deckhouse, which this webhook explicitly allows. Fail-open on an unusable baseline.
func rejectKubernetesVersionBelowMaxUsed(
	effective string, fromFallback bool, facts kubernetesVersionBaseline,
) (*kwhvalidating.ValidatorResult, error) {
	if facts.MaxUsed == "" {
		return nil, nil
	}
	floor := facts.MaxUsed

	maxUsed, err := parseVersion(floor)
	if err != nil {
		log.Warn("skipping the kubernetesVersion maxUsed guard: cannot parse the version floor",
			slog.String("value", floor), log.Err(err))
		return nil, nil
	}
	target, err := parseVersion(effective)
	if err != nil {
		log.Warn("skipping the kubernetesVersion maxUsed guard: cannot parse the target version",
			slog.String("value", effective), log.Err(err))
		return nil, nil
	}

	if !hooks.KubernetesVersionBelowFloor(target, maxUsed) {
		return nil, nil
	}

	if fromFallback {
		return rejectResult(fmt.Sprintf(
			"clearing or deleting the ModuleConfig kubernetesVersion override would fall back to "+
				"ClusterConfiguration.kubernetesVersion %q, which is more than one minor below %q, "+
				"the highest version the cluster has ever run; such a downgrade is forbidden",
			effective, maxUsed.Original(),
		))
	}
	return rejectResult(fmt.Sprintf(
		"kubernetesVersion %q is more than one minor below %q, the highest version the cluster "+
			"has ever run; such a downgrade is forbidden",
		effective, maxUsed.Original(),
	))
}

// Presence, not value. On a read error it reports false, i.e. keeps validating
// ClusterConfiguration — the safe direction.
func moduleConfigOwnsKubernetesVersion(ctx context.Context, cli client.Client) bool {
	cfg := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: controlPlaneManagerModuleName}, cfg); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the control-plane-manager ModuleConfig, validating ClusterConfiguration.kubernetesVersion anyway", log.Err(err))
		}
		return false
	}
	return settingsKubernetesVersion(rawModuleConfigSettings(cfg)) != ""
}

// The schema enum is what keeps the value a string, so anything else here is "unset".
func settingsKubernetesVersion(settings map[string]interface{}) string {
	version, _ := settings["kubernetesVersion"].(string)
	return version
}

// ModuleConfig takes Default only; ClusterConfiguration keeps the older Automatic.
func isModuleConfigTrackDefault(version string) bool {
	return version == defaultKubernetesVersionSentinel
}

func isClusterConfigurationPinned(version string) bool {
	return version != "" &&
		version != automaticKubernetesVersion &&
		version != defaultKubernetesVersionSentinel
}

// No schema conversion, so a failure on an unrelated field cannot hide an existing pin.
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

// The floor is spec.maxUsedKubernetesVersion and nothing else: the chain this replaced also tried
// status.currentVersion, which drops the moment a legitimate downgrade lands and let a second one
// through. Shared with the ClusterConfiguration webhook so the two cannot disagree.
func (v *moduleConfigValidator) readKubernetesVersionFacts(ctx context.Context) kubernetesVersionBaseline {
	secret, _ := v.readClusterConfigurationSecret(ctx)
	return kubernetesVersionBaselineFor(ctx, v.client, secret)
}

func (v *moduleConfigValidator) readClusterConfigurationSecret(ctx context.Context) (*v1.Secret, bool) {
	secret := &v1.Secret{}
	if err := v.client.Get(ctx, client.ObjectKey{
		Name:      clusterConfigurationSecretName,
		Namespace: kubeSystemNamespace,
	}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("skipping the kubernetesVersion fallback guard: cannot read the d8-cluster-configuration secret", log.Err(err))
		}
		return nil, false
	}
	return secret, true
}

// Used only when an MC pin is being cleared. ok=false is fail-open.
func (v *moduleConfigValidator) readRawClusterConfigurationVersion(ctx context.Context) (string, bool) {
	secret, ok := v.readClusterConfigurationSecret(ctx)
	if !ok {
		return "", false
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], cc); err != nil {
		return "", false
	}
	return cc.KubernetesVersion, true
}

// Forbids deleting kube-system/d8-cluster-kubernetes: it is the only durable record of
// maxUsedKubernetesVersion, and update-observer would recreate it without the history. DELETE only —
// hand edits of data.spec are rewritten on the next reconcile.
func clusterKubernetesConfigMapHandler() http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, ar *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		// Re-checked despite the rule's selectors: it runs with failurePolicy: Fail over kube-system
		// configmaps, so a selector that stopped matching would make them all undeletable.
		if ar.Operation == model.OperationDelete &&
			ar.Name == clusterKubernetesConfigMapName && ar.Namespace == kubeSystemNamespace {
			return rejectResult("It is forbidden to delete configmap d8-cluster-kubernetes")
		}

		return allowResult(nil)
	})

	wh, _ := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "cluster-kubernetes-configmap-validator",
		Validator: validator,
		Logger:    nil,
		Obj:       &v1.ConfigMap{},
	})

	return kwhhttp.MustHandlerFor(kwhhttp.HandlerConfig{Webhook: wh, Logger: nil})
}
