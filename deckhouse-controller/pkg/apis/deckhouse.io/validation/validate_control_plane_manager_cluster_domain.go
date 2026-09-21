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

	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
)

// Rejects a first write that disagrees with ClusterConfiguration, and any clearing that would move
// the domain. Changing an already set domain stays allowed. Fail-open on an unreadable Secret.
func (v *moduleConfigValidator) validateControlPlaneManagerClusterDomain(
	ctx context.Context, newSettings, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	newDomain := settingsClusterDomain(newSettings)
	oldDomain := settingsClusterDomain(oldSettings)

	if newDomain == oldDomain {
		return nil, nil
	}

	if newDomain == "" {
		return v.rejectClusterDomainRemoval(ctx, oldDomain, func(ccDomain string) string {
			return fmt.Sprintf(
				"clearing network.clusterDomain would change the cluster domain from %q to %q, restarting "+
					"kube-apiserver with a different --service-account-issuer and invalidating every token "+
					"in the cluster; set clusterDomain: %q in ClusterConfiguration before clearing this field",
				oldDomain, ccDomain, oldDomain)
		})
	}

	if oldDomain != "" {
		return nil, nil
	}

	ccDomain, ok := v.readRawClusterConfigurationDomain(ctx)
	if !ok || ccDomain == "" || ccDomain == newDomain {
		return nil, nil
	}

	return rejectResult(fmt.Sprintf(
		"clusterDomain %q does not match the cluster's current ClusterConfiguration.clusterDomain %q; "+
			"migrating the setting must keep the value the cluster already runs with",
		newDomain, ccDomain))
}

// Deleting the ModuleConfig drops its clusterDomain: rejected when that moves the domain, since a new
// --service-account-issuer invalidates every token in the cluster.
func (v *moduleConfigValidator) validateControlPlaneManagerClusterDomainDelete(
	ctx context.Context, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	domain := settingsClusterDomain(oldSettings)
	return v.rejectClusterDomainRemoval(ctx, domain, func(ccDomain string) string {
		return fmt.Sprintf(
			"deleting this ModuleConfig would change the cluster domain from %q to %q, restarting "+
				"kube-apiserver with a different --service-account-issuer and invalidating every token in "+
				"the cluster; set clusterDomain: %q in ClusterConfiguration before deleting this setting",
			domain, ccDomain, domain)
	})
}

// Shared by the delete and clear paths: rejects only when dropping the domain here would change what
// the cluster resolves today.
func (v *moduleConfigValidator) rejectClusterDomainRemoval(
	ctx context.Context, domain string, message func(ccDomain string) string,
) (*kwhvalidating.ValidatorResult, error) {
	if domain == "" {
		return nil, nil
	}

	ccDomain, ok := v.readRawClusterConfigurationDomain(ctx)
	if !ok {
		return nil, nil
	}
	if ccDomain == "" {
		ccDomain = hooks.DefaultClusterDomain
	}
	if ccDomain == domain {
		return nil, nil
	}

	return rejectResult(message(ccDomain))
}

// A missing map, group or non-string value all read as "not set".
func settingsClusterDomain(settings map[string]interface{}) string {
	network, _ := settings["network"].(map[string]interface{})
	domain, _ := network["clusterDomain"].(string)
	return domain
}

// ok=false is fail-open, so an unreadable Secret does not turn a first write into a mismatch.
func (v *moduleConfigValidator) readRawClusterConfigurationDomain(ctx context.Context) (string, bool) {
	secret, ok := v.readClusterConfigurationSecret(ctx)
	if !ok {
		return "", false
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], cc); err != nil {
		return "", false
	}

	return cc.ClusterDomain, true
}
