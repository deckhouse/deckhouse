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
	kubeSystemNamespace            = "kube-system"
	clusterConfigurationDataKey    = "cluster-configuration.yaml"
	clusterKubernetesStatusDataKey = "status"

	// automaticKubernetesVersion is the sentinel meaning "let Deckhouse pick the version".
	automaticKubernetesVersion = "Automatic"
)

// clusterKubernetesStatus is the subset of ConfigMap d8-cluster-kubernetes data.status
// needed for admission. Written by update-observer.
//
// The tag is `json`, not `yaml`: sigs.k8s.io/yaml converts YAML to JSON and then uses
// encoding/json, so a `yaml` tag would be ignored here and the field would only match by
// encoding/json's case-insensitive fallback.
type clusterKubernetesStatus struct {
	AvailableVersions []string `json:"availableVersions"`
}

// validateControlPlaneManagerKubernetesVersion guards ModuleConfig control-plane-manager's
// kubernetesVersion against membership in status.availableVersions from ConfigMap
// kube-system/d8-cluster-kubernetes (the set update-observer publishes as Supported[maxUsed-1:]).
// Explicit versions are also checked against module compatibility (validateKubernetesVersion).
//
// An explicit "Automatic" is deliberately exempt from membership. It means "track the Deckhouse
// default", and that path cannot run away: effective_kubernetes_version.go refuses to unbump below
// maxUsed-1, which is documented behaviour ("if the stable version is more than 1 minor below the
// maximum ever used, the version is not changed automatically") and is signalled by
// D8ControlPlaneDefaultVersionDrift. Rejecting it here would contradict that contract and leave
// clusters pinned high unable to ever hand control back to Deckhouse.
//
// Removing the setting (or deleting the ModuleConfig) is a different matter: resolution then falls
// back to the deprecated ClusterConfiguration field, whose leftover value from bootstrap can be
// arbitrarily stale, so the future effective version is resolved and membership-checked. Note this
// applies when the previous value was "Automatic" too — under presence-wins resolution, dropping
// the field does change which document owns the version.
//
// TODO(kubernetesVersion-deprecation): T+2 remove — that fallback branch: the `fromFallback`
// flag, the two reject messages mentioning ClusterConfiguration, and
// readRawClusterConfigurationVersion all go away with the field. Clearing the setting will then simply mean "use the Deckhouse default",
// which needs no cross-document lookup. Keep the availableVersions and maxUsed checks: after the
// removal this webhook is the only guard against a downgrade.
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
	case newVersion == automaticKubernetesVersion:
		// Handing the choice back to Deckhouse — self-limiting, see the doc comment above.
		return nil, nil
	case newVersion != "":
		effective = newVersion
	case oldVersion != "":
		// Clearing or deleting the setting: effective falls back to CC, then the Deckhouse default.
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
		// Never set on either side (HV-06, HV-07).
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

	// The maxUsed floor runs unconditionally, not only when availableVersions is missing.
	//
	// It covers two different gaps. First, status.availableVersions only exists once update-observer
	// has published it — not on a fresh cluster, not right after the ConfigMap was recreated, not
	// while status is empty or corrupt. Second, even when the list is present it can stop encoding
	// "no more than one minor below maxUsed": VersionSettings.Available returns the whole supported
	// list when maxUsed is not found in it, which happens after a Deckhouse downgrade or an edition
	// switch. Membership alone would then accept a pin far below the version the cluster has run.
	//
	// The two checks cannot contradict each other in the normal case, because the supported list is
	// contiguous in every edition's version_map, so its "one index below maxUsed" and this "one
	// minor below maxUsed" are the same version.
	if res, err := v.rejectKubernetesVersionBelowMaxUsed(ctx, effective, fromFallback); res != nil || err != nil {
		return res, err
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

// rejectKubernetesVersionBelowMaxUsed is the availableVersions guard's stand-in for the window
// before update-observer publishes status. It enforces only the one rule that matters there —
// never land more than one minor below the highest version the cluster has ever run — using
// maxUsedControlPlaneKubernetesVersion from the d8-cluster-configuration Secret.
//
// Deliberately not validateKubernetesVersionDowngrade: that one resolves "Automatic" on both
// sides and forbids handing control back to Deckhouse, which is a contract this webhook
// explicitly allows. Here effective is always an already-resolved explicit version, so a plain
// floor check is both correct and easier to reason about.
//
// Fail-open on a missing/unreadable/unparsable baseline, matching every other guard in this file.
func (v *moduleConfigValidator) rejectKubernetesVersionBelowMaxUsed(
	ctx context.Context, effective string, fromFallback bool,
) (*kwhvalidating.ValidatorResult, error) {
	baseline, ok := v.readKubernetesVersionBaseline(ctx)
	if !ok || !baseline.MaxUsedSet || baseline.MaxUsed == "" {
		return nil, nil
	}

	maxUsed, err := parseVersion(baseline.MaxUsed)
	if err != nil {
		log.Warn("skipping the kubernetesVersion maxUsed guard: cannot parse maxUsedControlPlaneKubernetesVersion",
			slog.String("value", baseline.MaxUsed), log.Err(err))
		return nil, nil
	}
	target, err := parseVersion(effective)
	if err != nil {
		return nil, nil
	}

	// Allowed floor is maxUsed minus one minor. Written as an addition on the target so the
	// uint64 minor never underflows on a 1.0-style version.
	switch {
	case target.Major() > maxUsed.Major():
		return nil, nil
	case target.Major() == maxUsed.Major() && target.Minor()+1 >= maxUsed.Minor():
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

// moduleConfigOwnsKubernetesVersion reports whether ModuleConfig control-plane-manager carries an
// explicit kubernetesVersion. Presence — not value — decides which document owns the version, so
// an explicit "Automatic" counts too (see resolveTargetKubernetesVersion in
// global-hooks/discovery/cluster_configuration.go).
//
// TODO(kubernetesVersion-deprecation): T+2 remove — only the ClusterConfiguration webhook calls this, to decide
// whether it should stand down. Once ClusterConfiguration has no kubernetesVersion there is
// nothing to stand down from, and this helper goes with it.
//
// Used by the ClusterConfiguration webhook to skip validating a field that no longer has any
// effect. On a read error it reports false, i.e. keeps validating ClusterConfiguration — the
// pre-migration behaviour, which is the safe direction for a guard.
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

func settingsKubernetesVersion(settings map[string]interface{}) string {
	if settings == nil {
		return ""
	}
	version, _ := settings["kubernetesVersion"].(string)
	return version
}

// isPinnedKubernetesVersion reports whether a kubernetesVersion value names a concrete version.
// Only ever applied to the ClusterConfiguration value here: for the ModuleConfig setting what
// matters is presence, not pinning (see validateControlPlaneManagerKubernetesVersion).
func isPinnedKubernetesVersion(version string) bool {
	return version != "" && version != automaticKubernetesVersion
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
func (v *moduleConfigValidator) readAvailableKubernetesVersions(ctx context.Context) ([]string, bool) {
	cm := &v1.ConfigMap{}
	if err := v.client.Get(ctx, client.ObjectKey{
		Name:      clusterKubernetesConfigMapName,
		Namespace: kubeSystemNamespace,
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

// readKubernetesVersionBaseline returns the version bookkeeping control-plane-manager keeps in the
// d8-cluster-configuration Secret. ok=false means fail-open (missing/unreadable).
func (v *moduleConfigValidator) readKubernetesVersionBaseline(ctx context.Context) (kubernetesVersionBaseline, bool) {
	secret, ok := v.readClusterConfigurationSecret(ctx)
	if !ok {
		return kubernetesVersionBaseline{}, false
	}
	return kubernetesVersionBaselineFromSecret(secret), true
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

// readRawClusterConfigurationVersion returns the literal kubernetesVersion from the
// d8-cluster-configuration Secret. Used only when resolving fallback after clearing an MC pin.
// ok=false means fail-open.
func (v *moduleConfigValidator) readRawClusterConfigurationVersion(ctx context.Context) (string, bool) {
	secret, ok := v.readClusterConfigurationSecret(ctx)
	if !ok {
		return "", false
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data[clusterConfigurationDataKey], cc); err != nil {
		return "", false
	}
	return cc.KubernetesVersion, true
}
