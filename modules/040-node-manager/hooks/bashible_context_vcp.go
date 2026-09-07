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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/bashiblecontext"
	"github.com/deckhouse/deckhouse/pkg/log"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// bashibleExternalInputsVersion is the only inputs.yaml format this Deckhouse can read. It is the
// tenant half of bashibleapiserver.ExternalInputsVersion in control-plane-manager: a host that
// starts writing a newer format switches its older tenants off instead of feeding them fields
// they would misread.
const bashibleExternalInputsVersion = 1

const bashibleExternalInputsSecretName = "bashible-external-inputs"

const (
	bashibleContextSecretNamespace = "d8-cloud-instance-manager"
	bashibleContextSecretName      = "bashible-apiserver-context"
	// The key templates/bashible-apiserver/context-secret.yaml renders the context under.
	bashibleContextSecretKey = "input.yaml"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/bashible_context_vcp",
	// After get_crds (10) and order_bootstrap_token (20): the tenant nodeGroups and per-NG tokens
	// this hook folds into the context are set by those.
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "inputs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"d8-cloud-instance-manager"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{bashibleExternalInputsSecretName}},
			FilterFunc:   filterBashibleExternalInputs,
		},
		{
			// The context already published, so a run that cannot assemble a new one can put it
			// back instead of dropping the Secret from the chart. See keepPublishedContext.
			Name:       "published",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{bashibleContextSecretNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{bashibleContextSecretName}},
			FilterFunc:   filterPublishedBashibleContext,
		},
	},
}, handleBashibleContextVCP)

// bashibleExternalInputs mirrors the contract control-plane-manager publishes in
// bashible-apiserver/inputs.go. The json tags match the bashible input.yaml keys the values end
// up under, so the overlay below is a straight assignment.
type bashibleExternalInputs struct {
	Version int `json:"version"`

	Deckhouse               bashibleInputsDeckhouse        `json:"deckhouse"`
	PodSubnetNodeCIDRPrefix string                         `json:"podSubnetNodeCIDRPrefix"`
	ClusterDNSAddress       string                         `json:"clusterDNSAddress"`
	ClusterUUID             string                         `json:"clusterUUID"`
	APIServerEndpoints      []string                       `json:"apiserverEndpoints"`
	ClusterMasterEndpoints  []bashibleInputsMasterEndpoint `json:"clusterMasterEndpoints"`
	ALBVIP                  string                         `json:"albVIP"`
	KonnHost                string                         `json:"konnHost"`
	APIServerProxyCerts     bashibleInputsProxyCerts       `json:"apiserverProxyCerts"`
	KubernetesCA            string                         `json:"kubernetesCA"`
	AllowedBundles          []string                       `json:"allowedBundles"`
	PackagesProxy           map[string]interface{}         `json:"packagesProxy"`
}

type bashibleInputsDeckhouse struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	Edition string `json:"edition"`
}

type bashibleInputsProxyCerts struct {
	Crt string `json:"crt"`
	Key string `json:"key"`
}

type bashibleInputsMasterEndpoint struct {
	Address                string `json:"address"`
	KubeAPIPort            int    `json:"kubeApiPort"`
	RPPServerPort          int    `json:"rppServerPort"`
	RPPBootstrapServerPort int    `json:"rppBootstrapServerPort"`
}

func filterBashibleExternalInputs(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	return string(secret.Data["inputs.yaml"]), nil
}

func filterPublishedBashibleContext(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	return string(secret.Data[bashibleContextSecretKey]), nil
}

// handleBashibleContextVCP assembles the bashible context for a virtual control plane tenant and
// publishes it for templates/bashible-apiserver/context-secret.yaml to render.
//
// A run that cannot assemble a context never publishes a partial one: a context missing the CA,
// the proxy certificates or the bootstrap token passes schema validation and bootstraps nodes
// silently wrong. It republishes the context already in the cluster instead — see
// keepPublishedContext for why it cannot just publish nothing.
func handleBashibleContextVCP(ctx context.Context, input *go_hook.HookInput) error {
	if !nestedControlPlane(input) {
		return nil
	}

	snaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "inputs")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'inputs' snapshot: %w", err)
	}
	if len(snaps) == 0 || snaps[0] == "" {
		return keepPublishedContext(input, "the host has published no external inputs")
	}

	inputs, reason := usableBashibleExternalInputs(snaps[0])
	if inputs == nil {
		return keepPublishedContext(input, reason)
	}

	setPhase1Values(input, inputs)

	service, err := newBashibleContextService()
	if err != nil {
		return fmt.Errorf("build a client for the managed cluster: %w", err)
	}

	assembled, err := service.Build(ctx, service.ReadGlobals(ctx), readTenantNodeGroups(input))
	if err != nil {
		input.Logger.Warn("cannot assemble the bashible context", log.Err(err))
		return keepPublishedContext(input, "the context could not be assembled")
	}
	applyBashibleExternalInputs(assembled, inputs)
	if tokens := tenantBootstrapTokens(input); tokens != nil {
		assembled["bootstrapTokens"] = tokens
	}

	// clusterDomain is the one field left to the tenant: it comes from its own
	// d8-cluster-configuration and lands in every kubelet config, so an empty one is as bad as
	// a missing CA.
	if domain, _ := assembled["clusterDomain"].(string); domain == "" {
		return keepPublishedContext(input, "the cluster domain is not discovered yet")
	}

	raw, err := bashiblecontext.Marshal(assembled)
	if err != nil {
		return fmt.Errorf("marshal bashible input.yaml: %w", err)
	}

	input.Values.Set("nodeManager.internal.bashibleContext", string(raw))
	return nil
}

// keepPublishedContext republishes the context the tenant already runs on.
//
// Leaving the value unset is not a neutral act: the Secret drops out of the rendered chart and
// Helm deletes it, so a Deckhouse restart that coincides with an unreadable contract or an API
// hiccup would take the context away from bashible-apiserver and break every node that asks it
// for a bundle. A context one revision stale still describes a working cluster, so it wins over
// no context at all. Only a tenant that has never had one publishes nothing, and there the
// Secret does not exist yet, so nothing is lost.
func keepPublishedContext(input *go_hook.HookInput, reason string) error {
	snaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "published")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'published' snapshot: %w", err)
	}
	if len(snaps) == 0 || snaps[0] == "" {
		input.Logger.Warn("publishing no bashible context", slog.String("reason", reason))
		return nil
	}

	input.Logger.Warn("keeping the bashible context already published", slog.String("reason", reason))
	input.Values.Set("nodeManager.internal.bashibleContext", snaps[0])
	return nil
}

// usableBashibleExternalInputs returns the published inputs, or nil and a reason when the tenant
// must publish nothing.
func usableBashibleExternalInputs(raw string) (*bashibleExternalInputs, string) {
	inputs := new(bashibleExternalInputs)
	if err := yaml.Unmarshal([]byte(raw), inputs); err != nil {
		return nil, fmt.Sprintf("external inputs are unreadable: %v", err)
	}
	if inputs.Version != bashibleExternalInputsVersion {
		return nil, fmt.Sprintf("external inputs version %d is not supported, this Deckhouse reads version %d",
			inputs.Version, bashibleExternalInputsVersion)
	}
	// The facts a node cannot bootstrap without and the tenant cannot supply. A document
	// carrying the right version but not these would assemble into a context that validates and
	// bootstraps nodes wrong, which is worse than no context at all.
	if inputs.KubernetesCA == "" ||
		inputs.APIServerProxyCerts.Crt == "" || inputs.APIServerProxyCerts.Key == "" ||
		inputs.ALBVIP == "" {
		return nil, "external inputs carry no kubernetes CA, no api-proxy certificates or no ALB VIP"
	}
	return inputs, ""
}

// applyBashibleExternalInputs overwrites every field a tenant cannot derive from its own cluster
// with the value the host published. The host is authoritative for all of them, including the
// ones the tenant can put a value on: its own d8-deckhouse-version-info, d8-cluster-uuid and
// default/kubernetes EndpointSlice describe the tenant, not the virtual control plane that
// bootstraps its nodes.
func applyBashibleExternalInputs(assembled map[string]interface{}, inputs *bashibleExternalInputs) {
	assembled["deckhouse"] = map[string]interface{}{
		"channel": inputs.Deckhouse.Channel,
		"version": inputs.Deckhouse.Version,
		"edition": inputs.Deckhouse.Edition,
	}
	assembled["podSubnetNodeCIDRPrefix"] = inputs.PodSubnetNodeCIDRPrefix
	assembled["clusterDNSAddress"] = inputs.ClusterDNSAddress
	assembled["clusterUUID"] = inputs.ClusterUUID
	assembled["apiserverEndpoints"] = inputs.APIServerEndpoints
	assembled["clusterMasterEndpoints"] = inputs.ClusterMasterEndpoints
	assembled["apiserverProxyCerts"] = inputs.APIServerProxyCerts
	assembled["kubernetesCA"] = inputs.KubernetesCA
	assembled["allowedBundles"] = inputs.AllowedBundles
	assembled["packagesProxy"] = inputs.PackagesProxy
}

// setPhase1Values feeds the stock manual-bootstrap templates from the host inputs. A nested
// tenant cannot discover these itself: it has no apiserver pods, and its own EndpointSlice points at an internal ClusterIP, not the ALB the external node needs.
//
// Each endpoint carries one port so the template hasKey checks stay clean. The rpp bootstrap
// fetch takes the ALB VIP, not a hostname: minget parses a numeric IP.
func setPhase1Values(input *go_hook.HookInput, inputs *bashibleExternalInputs) {
	addresses := make([]string, 0, len(inputs.ClusterMasterEndpoints))
	endpoints := make([]map[string]any, 0, len(inputs.ClusterMasterEndpoints))
	for _, e := range inputs.ClusterMasterEndpoints {
		if e.KubeAPIPort != 0 {
			endpoints = append(endpoints, map[string]any{"address": e.Address, "kubeApiPort": e.KubeAPIPort})
			addresses = append(addresses, fmt.Sprintf("%s:%d", e.Address, e.KubeAPIPort))
		}
		if e.RPPServerPort != 0 {
			endpoints = append(endpoints, map[string]any{"address": e.Address, "rppServerPort": e.RPPServerPort})
		}
		if e.RPPBootstrapServerPort != 0 {
			endpoints = append(endpoints, map[string]any{"address": inputs.ALBVIP, "rppBootstrapServerPort": e.RPPBootstrapServerPort})
		}
	}

	if inputs.KonnHost != "" {
		endpoints = append(endpoints, map[string]any{"address": inputs.KonnHost})
	}

	input.Values.Set("nodeManager.internal.clusterMasterAddresses", addresses)
	input.Values.Set("nodeManager.internal.clusterMasterEndpoints", endpoints)
	input.Values.Set("nodeManager.internal.albVIP", inputs.ALBVIP)
	input.Values.Set("nodeManager.internal.kubernetesCA", inputs.KubernetesCA)
	input.Values.Set("nodeManager.internal.packagesProxy", inputs.PackagesProxy)
}

// readTenantNodeGroups returns the tenant's own NodeGroups (get_crds fills the value) to drive
// the context.
func readTenantNodeGroups(input *go_hook.HookInput) []map[string]interface{} {
	raw := input.Values.Get("nodeManager.internal.nodeGroups")
	if !raw.Exists() || raw.String() == "" {
		return nil
	}
	var ngs []map[string]interface{}
	if err := json.Unmarshal([]byte(raw.String()), &ngs); err != nil {
		input.Logger.Warn("cannot read tenant nodeGroups for the bashible context", log.Err(err))
		return nil
	}
	// TODO: node-controller resolves each NodeGroup's kubernetesVersion/cri.type, but it does not run in a tenant now
	kubernetesVersion := input.Values.Get("global.clusterConfiguration.kubernetesVersion").String()
	defaultCRI := input.Values.Get("global.clusterConfiguration.defaultCRI").String()
	if defaultCRI == "" {
		defaultCRI = "Containerd"
	}
	for _, ng := range ngs {
		if kubernetesVersion != "" {
			if v, ok := ng["kubernetesVersion"].(string); !ok || v == "" {
				ng["kubernetesVersion"] = kubernetesVersion
			}
		}
		cri, _ := ng["cri"].(map[string]interface{})
		if cri == nil {
			cri = map[string]interface{}{}
			ng["cri"] = cri
		}
		if t, ok := cri["type"].(string); !ok || t == "" {
			cri["type"] = defaultCRI
		}
	}
	return ngs
}

// tenantBootstrapTokens returns the per-NG tokens order_bootstrap_token generated, folded into
// the context.
func tenantBootstrapTokens(input *go_hook.HookInput) map[string]string {
	raw := input.Values.Get("nodeManager.internal.bootstrapTokens")
	if !raw.Exists() || raw.String() == "" {
		return nil
	}
	var tokens map[string]string
	if err := json.Unmarshal([]byte(raw.String()), &tokens); err != nil {
		input.Logger.Warn("cannot read tenant bootstrapTokens for the bashible context", log.Err(err))
		return nil
	}
	return tokens
}

var bashibleContextScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(bashibleContextScheme))
}

// newBashibleContextService builds a controller-runtime client for the cluster this Deckhouse
// manages. Hooks are snapshot-driven and have no such client; go_lib/bashiblecontext needs one
// because it is the same code node-controller runs from inside a Pod, and duplicating its reads
// as bindings is exactly the divergence moving it to go_lib was meant to prevent.
//
// The in-cluster config is the tenant's even though the Pod runs in the parent: its
// KUBERNETES_SERVICE_HOST is the tenant API and the ServiceAccount token it mounts is a tenant
// one, both set by control-plane-manager in the Deployment it writes.
func newBashibleContextService() (*bashiblecontext.Service, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}

	c, err := client.New(restCfg, client.Options{Scheme: bashibleContextScheme})
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}

	return &bashiblecontext.Service{Client: c}, nil
}
