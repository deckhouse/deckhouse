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

package v2

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

// The registry the cluster already pulls from, expressed in the new configuration.
//
// Published, never applied. The ModuleConfig belongs to whoever administers the cluster,
// and a module that writes its own configuration takes away the only place an operator can
// state intent. cni-cilium, facing the same problem, publishes a suggestion alongside and
// creates the object only when none exists — but that part does not carry over here: the
// module's default is to manage nothing, so creating a configuration would be the module
// deciding to take over the pull path of a cluster nobody asked it to touch.
//
// What this saves is not typing but a transcription. The address, path, scheme,
// certificate authority and credentials of the registry a running cluster pulls from are
// spread across a secret and a docker configuration blob, and retyping them by hand is
// where a truncated path or the wrong authority comes from — on the one setting that
// decides whether the cluster can pull at all.
//
// It is offered before anyone asks the module to manage anything, and that timing is the
// whole design: the configuration schema refuses `mode: Managed` with no source of images,
// so there is no state in which someone has turned the module on and left the upstream
// blank for something to fill in later. By the time `mode: Managed` is expressible, the
// upstream has to already be written — so it is written down in advance, where publishing
// it changes nothing.
const (
	// DeckhouseRegistrySecretName holds the resolved legacy configuration: what the
	// cluster actually pulls with, rather than what any ModuleConfig asks for, which is
	// why it is the source here.
	DeckhouseRegistrySecretName = "deckhouse-registry"

	// SuggestedConfigSecretName is where the derived configuration is published for an
	// operator to review and apply.
	//
	// A Secret rather than a ConfigMap, unlike the equivalent in cni-cilium, for one
	// reason: this document carries the registry password. Leaving out the credentials
	// would take away most of what the suggestion is worth — they are the part that is
	// awkward to dig out by hand — so the container has to be the one that holds
	// credentials.
	SuggestedConfigSecretName = "registry-suggested-config"

	// SuggestedConfigKey is the entry inside it, named so that reading it out and
	// piping it into kubectl is the obvious thing to do.
	SuggestedConfigKey = "registry-mc.yaml"

	moduleConfigName = "registry"

	registrySecretSnapName = "deckhouse-registry"
	moduleConfigSnapName   = "module-config"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// Not OnBeforeHelm: this produces an object for a person to read, not values for
		// a template. Driven by the objects it derives from, so that it appears as soon
		// as a cluster's registry configuration is known and follows it afterwards.
		Queue: "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       registrySecretSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{DeckhouseRegistrySecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterDeckhouseRegistry,
			},
			{
				Name:       moduleConfigSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{moduleConfigName},
				},
				FilterFunc: filterModuleConfig,
			},
		},
	},
	handleImport,
)

func filterDeckhouseRegistry(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the secret: %w", err)
	}

	var config deckhouse_registry.Config
	config.FromSecretData(secret.Data)
	return config, nil
}

// moduleConfigFacts is what this needs to know about the module's own ModuleConfig:
// whether it exists at all, and whether someone has already configured the part that
// would be suggested.
type moduleConfigFacts struct {
	HasPrimary bool `json:"hasPrimary"`
}

func filterModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	settings, _, err := unstructured.NestedMap(obj.Object, "spec", "settings")
	if err != nil {
		return nil, fmt.Errorf("reading the registry ModuleConfig settings: %w", err)
	}

	_, hasPrimary := settings["primary"]
	return moduleConfigFacts{HasPrimary: hasPrimary}, nil
}

func handleImport(_ context.Context, input *go_hook.HookInput) error {
	legacy, err := helpers.SnapshotToSingle[deckhouse_registry.Config](input, registrySecretSnapName)
	if err != nil {
		// Nothing to derive from yet. Not an error: a cluster still coming up has no
		// resolved registry configuration, and this runs again when it does.
		return nil
	}

	upstream, err := upstreamFromLegacy(legacy)
	if err != nil {
		input.Logger.Warn("cannot express the existing registry configuration in the new form",
			"error", err.Error())
		return nil
	}
	if upstream == nil {
		return nil
	}

	configured := false
	if facts, err := helpers.SnapshotToSingle[moduleConfigFacts](input, moduleConfigSnapName); err == nil {
		configured = facts.HasPrimary
	}

	if decideSuggestion(configured) == suggestionWithdraw {
		input.PatchCollector.Delete("v1", "Secret", "d8-system", SuggestedConfigSecretName)
		return nil
	}

	document, err := suggestedModuleConfig(upstream)
	if err != nil {
		return err
	}

	input.PatchCollector.CreateOrUpdate(&v1core.Secret{
		TypeMeta: v1meta.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: v1meta.ObjectMeta{
			Name:      SuggestedConfigSecretName,
			Namespace: "d8-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "registry",
			},
			Annotations: map[string]string{
				"registry.deckhouse.io/description": "The cluster's existing registry configuration, " +
					"expressed for the controller-based implementation. Read it, review it, and apply it " +
					"before setting 'implementation: V2'. Holds the registry password.",
			},
		},
		Type:       v1core.SecretTypeOpaque,
		StringData: map[string]string{SuggestedConfigKey: document},
	})
	return nil
}

// suggestionOutcome is what to do with the suggestion secret.
type suggestionOutcome int

const (
	// suggestionPublish writes the suggestion, which is the state of a cluster not yet configured
	// for this implementation.
	suggestionPublish suggestionOutcome = iota

	// suggestionWithdraw removes it.
	suggestionWithdraw
)

// decideSuggestion answers whether the suggestion still has a reader.
//
// The whole of the converter's idempotency lives in this one answer, which is why it is a function
// rather than a condition inline: the suggestion exists to be applied, and once it has been — once the
// operator's own ModuleConfig names a primary — a converter that kept offering an alternative would be
// arguing with the answer it asked for. So a second pass over its own output does nothing but clean up
// after itself.
//
// Deliberately not "is their config identical to what we would suggest": an operator who changed the
// address, the credentials or the path has answered the question, and being told their answer differs
// from the machine's is not help.
func decideSuggestion(operatorNamedAPrimary bool) suggestionOutcome {
	if operatorNamedAPrimary {
		return suggestionWithdraw
	}
	return suggestionPublish
}

// suggestedModuleConfig renders the whole ModuleConfig, not only the fragment.
//
// So that applying it is one command rather than an editing exercise, and so that what an
// operator reads is the object they will end up with.
func suggestedModuleConfig(upstream map[string]any) (string, error) {
	encoded, err := yaml.Marshal(moduleConfigObject(upstream))
	if err != nil {
		return "", fmt.Errorf("rendering the suggested ModuleConfig: %w", err)
	}
	return string(encoded), nil
}

// moduleConfigObject is the document an operator would apply. Built as an object rather
// than a string so that what is published is exactly what the API would accept.
func moduleConfigObject(upstream map[string]any) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": moduleConfigName,
		},
		"spec": map[string]any{
			"enabled": true,
			"version": 1,
			"settings": map[string]any{
				// Left at its default of Unmanaged deliberately. This object records
				// where images come from; having the module take over the pull path is a
				// separate decision, and one an operator makes explicitly by setting
				// `mode: Managed`.
				//
				// The schema refuses settings without that mode, precisely so this cannot
				// be mistaken for a configuration that is in effect — which is why the
				// suggestion carries the mode explicitly rather than omitting it.
				"mode": "Managed",
				"primary": map[string]any{
					"upstream": upstream,
				},
			},
		},
	}
}

// upstreamFromLegacy turns the resolved legacy configuration into the new shape.
//
// Returns nothing when the legacy configuration names the in-cluster registry: that
// address is not an upstream but the thing the new implementation itself serves, and
// carrying it across would tell the cluster to pull its images from itself.
func upstreamFromLegacy(legacy deckhouse_registry.Config) (map[string]any, error) {
	if legacy.Address == "" {
		return nil, nil
	}
	if strings.HasPrefix(legacy.Address, "registry.d8-system.svc") {
		return nil, nil
	}

	scheme := strings.ToUpper(legacy.Scheme)
	if scheme == "" {
		scheme = "HTTPS"
	}

	upstream := map[string]any{
		"scheme": scheme,
		"host":   legacy.Address,
	}
	if path := strings.TrimSuffix(legacy.Path, "/"); path != "" {
		upstream["path"] = path
	}
	if legacy.CA != "" {
		upstream["ca"] = legacy.CA
	}

	username, password, err := registry_helpers.CredsFromDockerCfg(legacy.DockerConfig, legacy.Address)
	if err != nil {
		return nil, fmt.Errorf("reading the registry credentials: %w", err)
	}
	if username != "" {
		auth := map[string]any{"username": username}
		if password != "" {
			auth["password"] = password
		}
		upstream["auth"] = auth
	}

	return upstream, nil
}
