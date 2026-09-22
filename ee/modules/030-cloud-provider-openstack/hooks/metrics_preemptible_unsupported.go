/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// This hook exports the OpenStackPreemptibleUnsupported metric — one series per CloudEphemeral
// NodeGroup that references an OpenStackInstanceClass with spec.preemptible: true but is running
// where the raw Nova `preemptible` tag cannot reach a working preemption mechanism.
//
// Two reasons are recognised:
//   - "mcm"         — the NodeGroup is on the MCM engine, which has no raw-tag equivalent
//     (`machine-controller-manager`'s OpenStackMachineClass has no field for it).
//   - "non-selectel" — the NodeGroup is on CAPI, but the cluster's connection.authURL does not
//     point at Selectel; on other OpenStack providers Nova accepts the tag but ignores it.
//
// The metric is 1 for the unhealthy label pair and absent for the healthy one, so
// `sum by (node_group, reason) (d8_openstack_preemptible_unsupported) > 0` is enough to alert.
// This lives in the openstack module (not in node-manager) so that provider-specific behaviour
// stays inside the provider — node-manager remains generic.

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	preemptibleUnsupportedMetricName  = "d8_openstack_preemptible_unsupported"
	preemptibleUnsupportedMetricGroup = "d8_openstack_preemptible_unsupported"

	preemptibleUnsupportedReasonMCM         = "mcm"
	preemptibleUnsupportedReasonNonSelectel = "non-selectel"

	preemptibleSnapshotNodeGroups    = "node_groups"
	preemptibleSnapshotInstanceClass = "openstack_instance_classes"
	preemptibleSnapshotProvider      = "cloud_provider_secret"

	preemptibleUseMCMAnnotation      = "node.deckhouse.io/use-mcm"
	preemptibleCloudProviderSecret   = "d8-node-manager-cloud-provider"
	preemptibleCloudProviderSecretNs = "kube-system"

	preemptibleOpenStackClassKind = "OpenStackInstanceClass"

	engineMCM  = "MCM"
	engineCAPI = "CAPI"
)

// Three inputs feed the metric: NodeGroups that carry the intent, OpenStackInstanceClasses that
// carry the boolean, and the provider secret that carries the authURL. Any of the three changing
// can flip the alert on or off — the hook fires on every one of them, and the handler re-derives
// the full picture from the current snapshots.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cloud-provider-openstack/preemptible-unsupported",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         preemptibleSnapshotNodeGroups,
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeGroup",
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterPreemptibleNodeGroup,
		},
		{
			Name:                         preemptibleSnapshotInstanceClass,
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "OpenStackInstanceClass",
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   filterPreemptibleInstanceClass,
		},
		{
			Name:                         preemptibleSnapshotProvider,
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnSynchronization: ptr.To(true),
			NameSelector: &types.NameSelector{
				MatchNames: []string{preemptibleCloudProviderSecret},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{preemptibleCloudProviderSecretNs},
				},
			},
			FilterFunc: filterPreemptibleProviderSecret,
		},
	},
}, handlePreemptibleUnsupportedMetric)

type preemptibleNodeGroup struct {
	Name      string
	Engine    string
	ClassKind string
	ClassName string
	// Cloud is true only for CloudEphemeral NGs — those are the only ones the field applies to.
	Cloud bool
}

type preemptibleInstanceClass struct {
	Name        string
	Preemptible bool
}

type preemptibleProvider struct {
	AuthURL string
}

func filterPreemptibleNodeGroup(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, _, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || spec == nil {
		return preemptibleNodeGroup{Name: obj.GetName()}, nil
	}

	result := preemptibleNodeGroup{Name: obj.GetName()}

	nodeType, _, _ := unstructured.NestedString(spec, "nodeType")
	result.Cloud = nodeType == "CloudEphemeral"

	if kind, _, _ := unstructured.NestedString(spec, "cloudInstances", "classReference", "kind"); kind != "" {
		result.ClassKind = kind
	}
	if name, _, _ := unstructured.NestedString(spec, "cloudInstances", "classReference", "name"); name != "" {
		result.ClassName = name
	}

	// status.engine is the pin once node-controller has decided; before then we fall back to the
	// same heuristic node-controller uses on a fresh NG so the alert does not lag behind by one
	// reconcile after apply.
	if engine, _, _ := unstructured.NestedString(obj.Object, "status", "engine"); engine != "" {
		result.Engine = engine
	} else {
		// Openstack publishes both engine kinds; the default is CAPI unless the operator opts
		// into MCM via the annotation.
		if _, ok := obj.GetAnnotations()[preemptibleUseMCMAnnotation]; ok {
			result.Engine = engineMCM
		} else {
			result.Engine = engineCAPI
		}
	}
	return result, nil
}

func filterPreemptibleInstanceClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	preemptible, _, _ := unstructured.NestedBool(obj.Object, "spec", "preemptible")
	return preemptibleInstanceClass{Name: obj.GetName(), Preemptible: preemptible}, nil
}

// filterPreemptibleProviderSecret reads connection.authURL out of data["openstack"], which is a
// JSON blob (see modules/030-cloud-provider-openstack/templates/registration.yaml). A missing key
// yields "" — the handler then treats it as "not yet Selectel", not "not Selectel", so the metric
// does not spike on the first reconcile before the discovery hook publishes the connection block.
func filterPreemptibleProviderSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	raw, ok := secret.Data["openstack"]
	if !ok {
		return preemptibleProvider{}, nil
	}
	// data.openstack is a JSON tree — we only need connection.authURL, so parse into an
	// intermediate map rather than a full typed struct that would bind the hook to every
	// provider-tree change.
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		// Malformed secret is transient (mid-write) — treat as empty rather than fail the hook.
		return preemptibleProvider{}, nil
	}
	connection, _ := tree["connection"].(map[string]any)
	if connection == nil {
		return preemptibleProvider{}, nil
	}
	authURL, _ := connection["authURL"].(string)
	return preemptibleProvider{AuthURL: authURL}, nil
}

func handlePreemptibleUnsupportedMetric(_ context.Context, input *go_hook.HookInput) error {
	// Expire first — a NodeGroup that was unhealthy last pass but has since been fixed or deleted
	// must lose its timeseries in the same tick, so the alert clears without waiting for the
	// controller to restart or for another event.
	input.MetricsCollector.Expire(preemptibleUnsupportedMetricGroup)

	classes := map[string]bool{}
	for ic, err := range sdkobjectpatch.SnapshotIter[preemptibleInstanceClass](
		input.Snapshots.Get(preemptibleSnapshotInstanceClass)) {
		if err != nil {
			return fmt.Errorf("iterate %s: %w", preemptibleSnapshotInstanceClass, err)
		}
		classes[ic.Name] = ic.Preemptible
	}

	authURL := ""
	for provider, err := range sdkobjectpatch.SnapshotIter[preemptibleProvider](
		input.Snapshots.Get(preemptibleSnapshotProvider)) {
		if err != nil {
			return fmt.Errorf("iterate %s: %w", preemptibleSnapshotProvider, err)
		}
		authURL = provider.AuthURL
		// The secret name is unique — one entry at most, but iterate to unwrap the snapshot.
		break
	}
	nonSelectel := isNonSelectelAuthURL(authURL)

	for ng, err := range sdkobjectpatch.SnapshotIter[preemptibleNodeGroup](
		input.Snapshots.Get(preemptibleSnapshotNodeGroups)) {
		if err != nil {
			return fmt.Errorf("iterate %s: %w", preemptibleSnapshotNodeGroups, err)
		}
		if !ng.Cloud || ng.ClassKind != preemptibleOpenStackClassKind {
			continue
		}
		if !classes[ng.ClassName] {
			// Either the IC does not exist yet or preemptible is false — both are healthy from
			// the alert's perspective.
			continue
		}

		var reason string
		switch {
		case ng.Engine == engineMCM:
			reason = preemptibleUnsupportedReasonMCM
		case ng.Engine == engineCAPI && nonSelectel:
			reason = preemptibleUnsupportedReasonNonSelectel
		default:
			continue
		}

		input.MetricsCollector.Set(
			preemptibleUnsupportedMetricName,
			1,
			map[string]string{"node_group": ng.Name, "reason": reason},
			metrics.WithGroup(preemptibleUnsupportedMetricGroup),
		)
	}
	return nil
}

// isNonSelectelAuthURL returns true only when an authURL has been published AND it is clearly
// not Selectel. An empty authURL — the state during bootstrap before the discovery hook writes
// the secret — reads as "not yet Selectel" and does not raise the alert; the next tick, once
// the URL is known, will publish the metric if it turns out to be non-Selectel.
func isNonSelectelAuthURL(authURL string) bool {
	if authURL == "" {
		return false
	}
	lc := strings.ToLower(authURL)
	return !strings.Contains(lc, "selcloud.ru") && !strings.Contains(lc, "selectel")
}
