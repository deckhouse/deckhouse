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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// The three network parameters are being migrated from ClusterConfiguration into the network group
// of this ModuleConfig - see network.go in the same migration for the CC-side counterpart
// (validateUnsafeConfigChanges) that this guard's bypass annotation also controls.
const networkAllowUnsafeAnnotation = "deckhouse.io/allow-unsafe"

var networkFields = []string{"podSubnetCIDR", "serviceSubnetCIDR", "podSubnetNodeCIDRPrefix"}

// validateControlPlaneManagerNetwork guards the network group: once a parameter is set here it is
// immutable (changing or clearing it is the same "unsafe" operation validateUnsafeConfigChanges
// forbids on ClusterConfiguration), and setting it for the first time must agree with whatever
// ClusterConfiguration already says - otherwise "migrate the setting" would silently rewrite the
// network a running cluster uses. Fail-open on anything unreadable: this webhook runs with
// failurePolicy: Fail and is the sole publisher of both value keys (see cluster_network.go).
func (v *moduleConfigValidator) validateControlPlaneManagerNetwork(
	ctx context.Context, newSettings, oldSettings map[string]interface{}, annotations map[string]string,
) (*kwhvalidating.ValidatorResult, error) {
	newNetwork := settingsNetworkGroup(newSettings)
	oldNetwork := settingsNetworkGroup(oldSettings)

	var changed []string
	for _, field := range networkFields {
		if newNetwork[field] != oldNetwork[field] {
			changed = append(changed, field)
		}
	}
	if len(changed) == 0 || networkGuardBypassed(annotations) {
		return nil, nil
	}

	var cc map[string]string
	for _, field := range changed {
		oldValue, newValue := oldNetwork[field], newNetwork[field]

		if oldValue != "" {
			return rejectResult(fmt.Sprintf(
				"it is forbidden to change %s once set in ModuleConfig control-plane-manager "+
					"(was %q); this describes a running cluster's network and changing it is extremely "+
					"dangerous - recreate the cluster instead", field, oldValue))
		}

		// First write: it must agree with the deprecated ClusterConfiguration field when that field
		// is set, or "migrate the setting" would silently repoint a running cluster's network.
		if cc == nil {
			var ok bool
			cc, ok = v.readRawClusterConfigurationNetwork(ctx)
			if !ok {
				continue
			}
		}
		if ccValue := cc[field]; ccValue != "" && ccValue != newValue {
			return rejectResult(fmt.Sprintf(
				"%s %q does not match the cluster's current ClusterConfiguration.%s %q; "+
					"migrating the setting must keep the value the cluster already runs with",
				field, newValue, field, ccValue))
		}
	}

	return nil, nil
}

// moduleConfigOwnsNetworkField reports whether ModuleConfig control-plane-manager has a value for
// this network field. Presence, not value - mirrors moduleConfigOwnsKubernetesVersion. Used by the
// ClusterConfiguration webhook (validateUnsafeConfigChanges) so its own immutability check steps
// aside once this guard already owns the field; on a read error it reports false, i.e. keeps
// validating ClusterConfiguration, the safe direction.
func moduleConfigOwnsNetworkField(ctx context.Context, cli client.Client, field string) bool {
	cfg := new(v1alpha1.ModuleConfig)
	if err := cli.Get(ctx, client.ObjectKey{Name: controlPlaneManagerModuleName}, cfg); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("cannot read the control-plane-manager ModuleConfig, validating ClusterConfiguration network fields anyway", log.Err(err))
		}
		return false
	}
	return settingsNetworkGroup(rawModuleConfigSettings(cfg))[field] != ""
}

// networkGuardBypassed reports whether the ModuleConfig carries the same "allow-unsafe" annotation
// the ClusterConfiguration webhook honors, letting dhctl (or an operator who knows what they are
// doing) push a change validateControlPlaneManagerNetwork would otherwise reject.
func networkGuardBypassed(annotations map[string]string) bool {
	v, ok := annotations[networkAllowUnsafeAnnotation]
	return ok && v != "" && v != "null"
}

// settingsNetworkGroup reads the network group as a field->string map. A missing settings map,
// group or non-string field all come back as "not set" rather than an error - the schema keeps
// every one of these three fields a string.
func settingsNetworkGroup(settings map[string]interface{}) map[string]string {
	out := map[string]string{}
	group, _ := settings["network"].(map[string]interface{})
	for _, field := range networkFields {
		out[field], _ = group[field].(string)
	}
	return out
}

// readRawClusterConfigurationNetwork reads the three deprecated network fields straight out of the
// d8-cluster-configuration Secret. ok=false (fail-open, not "all empty") on any read/parse failure,
// so an unreadable Secret does not turn every first write into a forbidden mismatch.
func (v *moduleConfigValidator) readRawClusterConfigurationNetwork(ctx context.Context) (map[string]string, bool) {
	secret, ok := v.readClusterConfigurationSecret(ctx)
	if !ok {
		return nil, false
	}

	cc := new(clusterConfig)
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], cc); err != nil {
		return nil, false
	}

	return map[string]string{
		"podSubnetCIDR":           cc.PodSubnetCIDR,
		"serviceSubnetCIDR":       cc.ServiceSubnetCIDR,
		"podSubnetNodeCIDRPrefix": cc.PodSubnetNodeCIDRPrefix,
	}, true
}
