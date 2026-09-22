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
	"cmp"
	"context"
	"fmt"

	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
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
		if next, moves := v.clusterDomainAfterRemoval(ctx, oldDomain); moves {
			return rejectResult(clusterDomainRemovalMessage("clearing network.clusterDomain", oldDomain, next))
		}
		return nil, nil
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
	if next, moves := v.clusterDomainAfterRemoval(ctx, domain); moves {
		return rejectResult(clusterDomainRemovalMessage("deleting this ModuleConfig", domain, next))
	}
	return nil, nil
}

// Where the domain would move if dropped here, and whether that is a move at all. Fail-open on an
// unreadable Secret.
func (v *moduleConfigValidator) clusterDomainAfterRemoval(ctx context.Context, domain string) (string, bool) {
	if domain == "" {
		return "", false
	}

	ccDomain, ok := v.readRawClusterConfigurationDomain(ctx)
	if !ok {
		return "", false
	}

	ccDomain = cmp.Or(ccDomain, hooks.DefaultClusterDomain)
	return ccDomain, ccDomain != domain
}

func clusterDomainRemovalMessage(action, from, to string) string {
	return fmt.Sprintf(
		"%s would change the cluster domain from %q to %q, restarting kube-apiserver with a different "+
			"--service-account-issuer and invalidating every token in the cluster; set clusterDomain: %q "+
			"in ClusterConfiguration before that", action, from, to, from)
}

// Presence, not value. A read error reports false, which keeps ClusterConfiguration validated - the
// safe direction.
func moduleConfigOwnsClusterDomain(ctx context.Context, cli client.Client) bool {
	cfg := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: controlPlaneManagerModuleName}, cfg); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the control-plane-manager ModuleConfig, validating ClusterConfiguration.clusterDomain anyway", log.Err(err))
		}
		return false
	}
	return settingsClusterDomain(rawModuleConfigSettings(cfg)) != ""
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
