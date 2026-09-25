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
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Rejects a first write that disagrees with ClusterConfiguration, and any clearing that would move
// the domain. Changing an already set domain stays allowed. An unreadable Secret is a rejection:
// nothing may be concluded from it.
func (v *moduleConfigValidator) validateControlPlaneManagerClusterDomain(
	ctx context.Context, newSettings, oldSettings map[string]interface{},
) (*kwhvalidating.ValidatorResult, error) {
	newDomain := settingsClusterDomain(newSettings)
	oldDomain := settingsClusterDomain(oldSettings)

	if newDomain == oldDomain {
		return nil, nil
	}

	if newDomain == "" {
		if msg := v.clusterDomainRemovalRejection(ctx, "clearing network.clusterDomain", oldDomain); msg != "" {
			return rejectResult(msg)
		}
		return nil, nil
	}

	if oldDomain != "" {
		return nil, nil
	}

	ccDomain, err := v.clusterConfigurationDomain(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return rejectResult(clusterDomainUnverifiableMessage("setting network.clusterDomain"))
	}
	if ccDomain == "" || ccDomain == newDomain {
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
	if msg := v.clusterDomainRemovalRejection(ctx, "deleting this ModuleConfig", domain); msg != "" {
		return rejectResult(msg)
	}
	return nil, nil
}

// Rejection message for dropping the domain here, empty when that is safe. Three states matter:
// no Secret at all means the cluster never resolves from this ModuleConfig, while a Secret that
// cannot be read leaves the outcome unknown and must not be waved through.
func (v *moduleConfigValidator) clusterDomainRemovalRejection(ctx context.Context, action, domain string) string {
	if domain == "" {
		return ""
	}

	ccDomain, err := v.clusterConfigurationDomain(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ""
		}
		return clusterDomainUnverifiableMessage(action)
	}

	ccDomain = cmp.Or(ccDomain, hooks.DefaultClusterDomain)
	if ccDomain == domain {
		return ""
	}

	return clusterDomainRemovalMessage(action, domain, ccDomain)
}

func clusterDomainUnverifiableMessage(action string) string {
	return fmt.Sprintf(
		"%s cannot be verified: the d8-cluster-configuration Secret is unreadable, so it is unknown "+
			"whether this changes the cluster domain; retry once the Secret can be read", action)
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

// The deprecated domain. A NotFound error means the document does not exist at all, which is not
// the same as an unreadable one: callers must tell those apart.
func (v *moduleConfigValidator) clusterConfigurationDomain(ctx context.Context) (string, error) {
	secret := new(v1.Secret)
	if err := v.client.Get(ctx, client.ObjectKey{
		Name:      clusterConfigurationSecretName,
		Namespace: kubeSystemNamespace,
	}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the d8-cluster-configuration secret", log.Err(err))
		}
		return "", err
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], cc); err != nil {
		return "", fmt.Errorf("parse cluster-configuration.yaml: %w", err)
	}

	return cc.ClusterDomain, nil
}
