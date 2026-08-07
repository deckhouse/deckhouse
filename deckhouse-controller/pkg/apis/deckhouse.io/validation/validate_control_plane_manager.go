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
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	clusterKubernetesConfigMapName = "d8-cluster-kubernetes"
	clusterConfigurationSecretName = "d8-cluster-configuration"
	kubeSystemNamespace            = "kube-system"
	clusterConfigurationDataKey    = "cluster-configuration.yaml"
	clusterKubernetesStatusDataKey = "status"
	clusterKubernetesSpecDataKey   = "spec"

	// automaticKubernetesVersion is the ClusterConfiguration sentinel for "track Deckhouse
	// default". Not accepted in ModuleConfig, where Default is the only sentinel.
	automaticKubernetesVersion = "Automatic"
	// defaultKubernetesVersionSentinel is the ModuleConfig sentinel for "track Deckhouse default".
	defaultKubernetesVersionSentinel = "Default"
)

// clusterKubernetesStatus is the subset of ConfigMap d8-cluster-kubernetes data.status
// needed for admission. Written by update-observer.
//
// The tag is `json`, not `yaml`: sigs.k8s.io/yaml converts YAML to JSON and then uses
// encoding/json, so a `yaml` tag would be ignored here and the field would only match by
// encoding/json's case-insensitive fallback.
type clusterKubernetesStatus struct {
	AvailableVersions []string `json:"availableVersions"`
	// AutomaticVersion is the Kubernetes version this Deckhouse build treats as its default —
	// what "Default" resolves to. It is what deckhouseDefaultKubernetesVersion in the
	// d8-cluster-configuration Secret used to answer.
	AutomaticVersion string `json:"automaticVersion"`
}

// clusterKubernetesSpec is the subset of data.spec admission needs. Written by update-observer,
// which is the only writer of the whole object.
type clusterKubernetesSpec struct {
	// MaxUsedVersion is the highest Kubernetes minor the cluster has ever converged onto (it leads
	// the running control plane by one minor during a rollout; update-observer's controller.Spec
	// carries the exact definition). It is the floor a downgrade may not cross by more than one
	// minor, and it is monotonic — which is exactly why it is the only acceptable source:
	// currentVersion is a point in time and drops as soon as a legitimate downgrade lands, which
	// used to let a second downgrade straight through.
	MaxUsedVersion string `json:"maxUsedKubernetesVersion"`
}

// validateControlPlaneManagerKubernetesVersion guards ModuleConfig control-plane-manager's
// kubernetesVersion against membership in status.availableVersions from ConfigMap
// kube-system/d8-cluster-kubernetes (the set update-observer publishes as Supported[maxUsed-1:]).
// Explicit versions are also checked against module compatibility (validateKubernetesVersion).
//
// An explicit "Default" is deliberately exempt from membership.
// It means "track the Deckhouse default", and that path cannot run away:
// effective_kubernetes_version.go refuses to unbump below maxUsed-1.
//
// Removing the setting (or deleting the ModuleConfig) falls back to the deprecated
// ClusterConfiguration field, so the future effective version is resolved and membership-checked.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — drop CC fallback branch when clearing MC kubernetesVersion.
//
// Unchanged kubernetesVersion skips the check so edits to unrelated settings are not blocked by an
// orphaned pin that fell outside availableVersions after the ConfigMap appeared or Supported shrank.
//
// Fail-open on missing/empty ConfigMap status, empty availableVersions, or read errors: the
// ModuleConfig webhook runs with failurePolicy: Fail, and a transient outage must not lock out edits.
func (v *moduleConfigValidator) validateControlPlaneManagerKubernetesVersion(
	ctx context.Context, newSettings, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	newVersion, newIsString := settingsKubernetesVersion(newSettings)
	if !newIsString {
		return rejectResult("kubernetesVersion must be a string, for example \"1.35\" or \"Default\"" +
			" (an unquoted version is parsed as a number)")
	}
	// The old value is only a reference point; a historical non-string is treated as "unset" rather
	// than blocking an edit that may well be the fix for it.
	oldVersion, _ := settingsKubernetesVersion(oldSettings)

	if newVersion == oldVersion {
		return nil, nil
	}

	var (
		effective    string
		fromFallback bool
	)
	switch {
	case isModuleConfigTrackDefault(newVersion):
		// Handing the choice back to Deckhouse — self-limiting, see the doc comment above.
		// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
		log.Info("E2E-KV admission",
			"decision", "allow",
			"reason", "track-default",
			"newVersion", newVersion,
			"oldVersion", oldVersion,
		)
		return nil, nil
	case newVersion != "":
		effective = newVersion
	case oldVersion != "":
		// Clearing or deleting the setting: effective falls back to CC, then the Deckhouse default.
		ccVersion, ok := v.readRawClusterConfigurationVersion(ctx)
		if !ok {
			// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
			log.Info("E2E-KV admission",
				"decision", "allow",
				"reason", "clear-fail-open-no-cc",
				"oldVersion", oldVersion,
			)
			return nil, nil
		}
		if !isClusterConfigurationPinned(ccVersion) {
			// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
			log.Info("E2E-KV admission",
				"decision", "allow",
				"reason", "clear-fail-open-cc-unpinned",
				"oldVersion", oldVersion,
				"ccVersion", ccVersion,
			)
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
		// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
		log.Info("E2E-KV admission",
			"decision", "reject",
			"reason", "module-compatibility",
			"effective", effective,
			"fromFallback", fromFallback,
			"message", res.Message,
		)
		return res, nil
	}

	// Both remaining guards read the same two objects, so they read them once, together: the floor
	// and the membership list are two projections of the same ConfigMap, and rejecting a version
	// against one snapshot while accepting it against another would be indistinguishable from a bug.
	// Resolved after validateKubernetesVersion so the module-compatibility reject path still costs
	// no API reads.
	facts := v.readKubernetesVersionFacts(ctx)

	// The maxUsed floor runs unconditionally, not only when availableVersions is missing.
	// Membership alone can miss deep downgrades before status is published or when Supported
	// no longer encodes maxUsed-1 after a Deckhouse/edition change.
	if res, err := rejectKubernetesVersionBelowMaxUsed(effective, fromFallback, facts); res != nil || err != nil {
		if res != nil && !res.Valid {
			// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
			log.Info("E2E-KV admission",
				"decision", "reject",
				"reason", "below-maxUsed",
				"effective", effective,
				"fromFallback", fromFallback,
				"message", res.Message,
			)
		}
		return res, err
	}

	available := facts.AvailableVersions
	if len(available) == 0 {
		// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
		log.Info("E2E-KV admission",
			"decision", "allow",
			"reason", "fail-open-no-availableVersions",
			"effective", effective,
			"fromFallback", fromFallback,
		)
		return nil, nil
	}

	if !slices.Contains(available, effective) {
		// availableVersions is bounded on both ends, so a miss is not necessarily a downgrade:
		// a version newer than everything the release offers lands here too, and telling that
		// operator about "downgrading more than one minor" sends them looking for the wrong thing.
		reason := "downgrading more than one minor below the highest version the cluster has ever run is forbidden"
		if aboveEveryVersion(effective, available) {
			reason = "that version is newer than any version this Deckhouse release supports"
		}

		msg := ""
		if fromFallback {
			msg = fmt.Sprintf(
				"clearing or deleting the ModuleConfig kubernetesVersion override would fall back to "+
					"ClusterConfiguration.kubernetesVersion %q, which is not in the cluster's availableVersions %v; %s",
				effective, available, reason,
			)
		} else {
			msg = fmt.Sprintf(
				"kubernetesVersion %q is not in the cluster's availableVersions %v; %s",
				effective, available, reason,
			)
		}
		// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
		log.Info("E2E-KV admission",
			"decision", "reject",
			"reason", "not-in-availableVersions",
			"effective", effective,
			"fromFallback", fromFallback,
			"message", msg,
		)
		return rejectResult(msg)
	}
	// TODO(E2E-KV): temporary stand Info logs — remove before final PR (`rg E2E-KV`).
	log.Info("E2E-KV admission",
		"decision", "allow",
		"reason", "in-availableVersions",
		"effective", effective,
		"fromFallback", fromFallback,
	)
	return nil, nil
}

// rejectKubernetesVersionBelowMaxUsed enforces "never land more than one minor below maxUsed"
// using spec.maxUsedKubernetesVersion from the d8-cluster-kubernetes ConfigMap.
//
// Deliberately not validateKubernetesVersionDowngrade: that one resolves "Automatic" on both
// sides and forbids handing control back to Deckhouse, which is a contract this webhook
// explicitly allows. Here effective is always an already-resolved explicit version.
//
// Takes the resolved baseline rather than a client so the decision is a pure function of a single
// snapshot — the caller owns the reads, and this is unit-testable without an API server.
//
// Fail-open on a missing/unreadable/unparsable baseline, matching every other guard in this file.
func rejectKubernetesVersionBelowMaxUsed(
	effective string, fromFallback bool, facts kubernetesVersionBaseline,
) (*kwhvalidating.ValidatorResult, error) {
	if !facts.MaxUsedSet || facts.MaxUsed == "" {
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
		// A guard that switches itself off must say so — the other fail-open branch above logs,
		// and silence here made an unparsable pin look like an accepted one.
		log.Warn("skipping the kubernetesVersion maxUsed guard: cannot parse the target version",
			slog.String("value", effective), log.Err(err))
		return nil, nil
	}

	if !kubernetesVersionBelowFloor(target, maxUsed) {
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
// an explicit "Automatic" counts too.
//
// TODO(kubernetesVersion-deprecation): T+1 remove — only CC webhook uses this until CC field is gone.
//
// On a read error it reports false, i.e. keeps validating ClusterConfiguration — the safe direction.
func moduleConfigOwnsKubernetesVersion(ctx context.Context, cli client.Client) bool {
	cfg := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: controlPlaneManagerModuleName}, cfg); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the control-plane-manager ModuleConfig, validating ClusterConfiguration.kubernetesVersion anyway", log.Err(err))
		}
		return false
	}
	// Presence decides ownership, so a non-string value still means ModuleConfig owns the field —
	// its own webhook rejects it, and ClusterConfiguration must not silently take over meanwhile.
	version, isString := settingsKubernetesVersion(rawModuleConfigSettings(cfg))
	return version != "" || !isString
}

// aboveEveryVersion reports whether target is higher than every entry in available. Unparsable
// input answers false: the caller only uses this to pick a message, and the generic downgrade
// wording is the safer guess.
func aboveEveryVersion(target string, available []string) bool {
	targetV, err := parseVersion(target)
	if err != nil {
		return false
	}
	for _, raw := range available {
		v, err := parseVersion(raw)
		if err != nil {
			return false
		}
		if !targetV.GreaterThan(v) {
			return false
		}
	}
	return len(available) > 0
}

// settingsKubernetesVersion returns the kubernetesVersion setting. ok=false means the key is
// present but not a string.
//
// The distinction matters because spec.settings is x-kubernetes-preserve-unknown-fields: an
// unquoted `kubernetesVersion: 1.35` arrives as a number, and the enum in the schema does not
// always catch it first — validateCR returns before validateSettings when spec.enabled is false
// (go_lib/configtools/validator.go), yet result.Settings is already populated by then. Collapsing
// that to "" made the guard read it as "the field was cleared" and validate the ClusterConfiguration
// fallback instead of what the operator actually wrote.
func settingsKubernetesVersion(settings map[string]interface{}) (string, bool) {
	if settings == nil {
		return "", true
	}
	raw, present := settings["kubernetesVersion"]
	if !present || raw == nil {
		return "", true
	}
	version, isString := raw.(string)
	if !isString {
		return "", false
	}
	return version, true
}

// The two documents no longer share one predicate, because they no longer accept the same words.
// ModuleConfig takes Default only; ClusterConfiguration keeps Automatic, which predates Default
// there and cannot be removed without breaking existing documents.

// isModuleConfigTrackDefault reports the ModuleConfig sentinel for "track the Deckhouse default".
func isModuleConfigTrackDefault(version string) bool {
	return version == defaultKubernetesVersionSentinel
}

// isClusterConfigurationPinned reports whether the ClusterConfiguration value names a concrete
// version. For the ModuleConfig setting what matters is presence, not pinning
// (see validateControlPlaneManagerKubernetesVersion).
//
// Default is rejected here too even though the ClusterConfiguration schema does not accept it:
// this predicate decides whether the value is handed onward as a version, and an obvious
// non-version must never get through, schema or no schema.
func isClusterConfigurationPinned(version string) bool {
	return version != "" &&
		version != automaticKubernetesVersion &&
		version != defaultKubernetesVersionSentinel
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

// readKubernetesVersionFacts resolves every cluster-level fact this webhook needs — the maxUsed
// floor, the Deckhouse default and status.availableVersions — from one read of each object.
//
// The floor is the version the cluster must not land more than one minor below: the highest minor it
// has ever converged onto, and exactly one quantity answers that — spec.maxUsedKubernetesVersion of
// the cluster ConfigMap. The chain this replaced mixed it with two values that are not it:
// status.currentVersion is a point in time and drops the moment a legitimate downgrade lands, which
// silently lowered the floor and let a second downgrade through, and spec.desiredVersion is a
// declaration rather than a record.
//
// Shares kubernetesVersionBaselineFor with the ClusterConfiguration webhook on purpose: one
// resolution chain (ConfigMap, falling back per field to the d8-cluster-configuration Secret) means
// the two webhooks cannot disagree about the same window. Reading it once also means the floor and
// the membership list come from the same snapshot of the same object.
//
// The Secret is the migration fallback for the window between a Deckhouse upgrade and the DaemonSet
// rollout that first writes the ConfigMap key; a missing Secret is not an error, the baseline just
// keeps whatever the ConfigMap provided.
// TODO(kubernetesVersion-deprecation): T+1 remove — drop the Secret branch.
//
// Every field comes back unset when nothing at all is known — a cluster still bootstrapping, where
// there is no version to protect yet — and each guard fail-opens on that.
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

// clusterKubernetesConfigMapHandler forbids deleting kube-system/d8-cluster-kubernetes.
//
// After this release that ConfigMap is the only durable record of maxUsedKubernetesVersion, the
// floor that stops a cluster from being downgraded more than one minor below what it has ever run.
// update-observer recreates the object when it is deleted, but it recreates it from its own
// container environment — the history is gone, and the floor silently drops to the current
// version. The d8-cluster-configuration Secret carries the same protection for the same reason.
//
// Only DELETE. Hand edits of data.spec need no rule: update-observer rewrites the whole block on
// its next reconcile, which the delete of any key already triggers.
func clusterKubernetesConfigMapHandler() http.Handler {
	validator := kwhvalidating.ValidatorFunc(func(_ context.Context, ar *model.AdmissionReview, _ metav1.Object) (*kwhvalidating.ValidatorResult, error) {
		// The object is also narrowed by the rule's objectSelector/namespaceSelector in
		// modules/002-deckhouse/templates/admission/validation.yaml. Checking it here too costs one
		// comparison and bounds the damage of a mistake in that template: the rule runs with
		// failurePolicy: Fail over configmaps in kube-system, so a selector that stopped matching
		// would otherwise make every ConfigMap in that namespace undeletable.
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
