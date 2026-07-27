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

	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// validateControlPlaneManagerKubernetesVersion guards ModuleConfig control-plane-manager's
// kubernetesVersion setting against the same downgrade rules already enforced for
// ClusterConfiguration.kubernetesVersion (see validateKubernetesVersionDowngrade in
// validate_cluster_configuration.go), and validates an explicit version against module
// compatibility requirements (see validateKubernetesVersion).
//
// If the new settings don't set kubernetesVersion, the effective value defers to
// ClusterConfiguration, whose own admission webhook already guards edits made there.
//
// Known gap: *removing* an existing ModuleConfig override is not guarded. The new value is empty,
// so this returns early, while the effective version silently falls back to whatever
// ClusterConfiguration still holds — potentially a stale pin several minors below what the cluster
// runs. Deleting the ModuleConfig outright is likewise unguarded: that path is handled by
// validateDelete and never reaches this function.
func (v *moduleConfigValidator) validateControlPlaneManagerKubernetesVersion(
	ctx context.Context, newSettings, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	newVersion, _ := newSettings["kubernetesVersion"].(string)
	if newVersion == "" {
		return nil, nil
	}

	secret := &v1.Secret{}
	if err := v.client.Get(ctx, client.ObjectKey{Name: "d8-cluster-configuration", Namespace: "kube-system"}, secret); err != nil {
		// Fail open, deliberately. The ModuleConfig webhook runs with failurePolicy: Fail, so
		// returning an error here would reject every edit of ModuleConfig control-plane-manager for
		// as long as this read keeps failing — a transient API outage would lock out the very
		// config an operator needs to fix things. A missing Secret (dry-run, cluster not
		// bootstrapped yet) is an expected state; anything else is unexpected and worth a log line.
		if !apierrors.IsNotFound(err) {
			log.Warn("skipping the kubernetesVersion downgrade guard: cannot read the d8-cluster-configuration secret", log.Err(err))
		}

		return nil, nil
	}

	// validateKubernetesVersion/validateKubernetesVersionDowngrade are shared with the
	// ClusterConfiguration webhook (clusterConfigurationHandler), where they're chained as
	// kwhvalidating.Validators: they always return a non-nil result, Valid=true on success. That
	// differs from validateCommon's "nil,nil means allow" convention, so results are translated
	// below instead of being propagated directly.
	if newVersion != "Automatic" {
		res, err := validateKubernetesVersion(newVersion, v.moduleManager)
		if err != nil {
			return nil, err
		}
		if res != nil && !res.Valid {
			return res, nil
		}
	}

	oldVersion, _ := oldSettings["kubernetesVersion"].(string)
	if oldVersion == "" {
		cc := new(clusterConfig)
		if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], cc); err != nil {
			// Malformed/absent ClusterConfiguration is unrelated to this check — don't block on it.
			return nil, nil
		}
		oldVersion = cc.KubernetesVersion
	}
	if oldVersion == "" {
		return nil, nil
	}

	res, err := validateKubernetesVersionDowngrade(oldVersion, newVersion, kubernetesVersionBaselineFromSecret(secret))
	if err != nil {
		return nil, err
	}
	if res != nil && !res.Valid {
		return res, nil
	}
	return nil, nil
}
