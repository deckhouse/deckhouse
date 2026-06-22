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

// Package modulesource watches ModuleSource objects (excluding deckhouse-managed
// ones) and projects their real upstream registry into
// registry.internal.moduleSourceEntries, which the registry-config.yaml template
// ranges to populate the RegistryConfig CR consumed by the per-node agent.
//
// For ModuleSources that have not yet been rewritten by the webhook (no
// registry.deckhouse.io/upstream annotation) a backstop PatchWithMerge rewrites
// them to the local svc and captures the original upstream in the annotation —
// mirroring what the webhook does for new CREATE/UPDATE events.
package modulesource

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	// primarySvc is the in-cluster registry address; must match the webhook's PrimarySvc.
	primarySvc = "registry.d8-system.svc:5001"

	// upstreamAnnotation is the annotation that the webhook writes when it first
	// rewrites a ModuleSource's spec.registry to the local svc.
	upstreamAnnotation = "registry.deckhouse.io/upstream"

	msSnap = "modulesources"
	queue  = "/modules/registry/modulesource"

	pkiCAPath    = "registry.internal.pki.ca.cert"
	pkiUsersPath = "registry.internal.pki.users"
)

// pkiUser mirrors the Users slice stored under registry.internal.pki.users.
type pkiUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        queue,
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       msSnap,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "ModuleSource",
				// Exclude deckhouse-managed ModuleSources (heritage=deckhouse label).
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"deckhouse"},
						},
					},
				},
				FilterFunc: filterModuleSource,
			},
		},
	},
	handle,
)

// filterModuleSource extracts the fields we need from a ModuleSource object into
// an MSSnap.
func filterModuleSource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	annotations := obj.GetAnnotations()

	upstreamJSON := ""
	if annotations != nil {
		upstreamJSON = annotations[upstreamAnnotation]
	}

	repo, _, _ := unstructured.NestedString(obj.Object, "spec", "registry", "repo")
	scheme, _, _ := unstructured.NestedString(obj.Object, "spec", "registry", "scheme")
	ca, _, _ := unstructured.NestedString(obj.Object, "spec", "registry", "ca")
	dockerCfg, _, _ := unstructured.NestedString(obj.Object, "spec", "registry", "dockerCfg")

	return MSSnap{
		Name:          name,
		RepoInSpec:    repo,
		UpstreamJSON:  upstreamJSON,
		SpecScheme:    scheme,
		SpecCA:        ca,
		SpecDockerCfg: dockerCfg,
	}, nil
}

func handle(_ context.Context, input *go_hook.HookInput) error {
	snaps, err := helpers.SnapshotToList[MSSnap](input, msSnap)
	if err != nil {
		return fmt.Errorf("get modulesource snapshot: %w", err)
	}

	entries, toPatch := projectEntries(snaps)

	helpers.NewValuesAccessor[[]Entry](input, "registry.internal.moduleSourceEntries").Set(entries)

	if len(toPatch) == 0 {
		return nil
	}

	// Read PKI values for the backstop rewrite.
	moduleCACert := input.Values.Get(pkiCAPath).String()

	localDockerCfg, err := buildLocalDockerCfg(input)
	if err != nil {
		// PKI not yet populated (very early reconcile): skip backstop silently;
		// the hook will re-run when the pki hook writes the values.
		input.Logger.Warn("modulesource backstop: PKI not ready, skipping", "err", err)
		return nil
	}

	// For each not-yet-rewritten ModuleSource, build a snapshot of the real spec
	// and apply a merge patch that rewrites it to the local svc (mirror of webhook).
	snapsByName := make(map[string]MSSnap, len(snaps))
	for _, s := range snaps {
		snapsByName[s.Name] = s
	}

	for _, name := range toPatch {
		s, ok := snapsByName[name]
		if !ok {
			continue
		}

		// Capture the original upstream spec as the annotation value.
		type upstreamCapture struct {
			Scheme    string `json:"scheme,omitempty"`
			Repo      string `json:"repo"`
			CA        string `json:"ca,omitempty"`
			DockerCfg string `json:"dockerCfg,omitempty"`
		}
		captured, err := json.Marshal(upstreamCapture{
			Scheme:    s.SpecScheme,
			Repo:      s.RepoInSpec,
			CA:        s.SpecCA,
			DockerCfg: s.SpecDockerCfg,
		})
		if err != nil {
			return fmt.Errorf("backstop: marshal upstream for %q: %w", name, err)
		}

		patch := map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					upstreamAnnotation: string(captured),
				},
			},
			"spec": map[string]any{
				"registry": map[string]any{
					"repo":      primarySvc + "/" + s.RepoInSpec,
					"scheme":    "HTTPS",
					"ca":        moduleCACert,
					"dockerCfg": localDockerCfg,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1alpha1", "ModuleSource", "", name)
	}

	return nil
}

// buildLocalDockerCfg constructs a base64-encoded docker config JSON for the
// local ReadOnly registry user, mirroring creds.buildDockerCfg in the webhook.
func buildLocalDockerCfg(input *go_hook.HookInput) (string, error) {
	usersRaw := input.Values.Get(pkiUsersPath)
	if !usersRaw.Exists() {
		return "", fmt.Errorf("pki users not yet populated")
	}

	var users []pkiUser
	if err := json.Unmarshal([]byte(usersRaw.Raw), &users); err != nil {
		return "", fmt.Errorf("parse pki users: %w", err)
	}

	var ro *pkiUser
	for i := range users {
		if users[i].Role == "ReadOnly" {
			ro = &users[i]
			break
		}
	}
	if ro == nil {
		return "", fmt.Errorf("no ReadOnly user in pki users")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(ro.Name + ":" + ro.Password))

	type authEntry struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Auth     string `json:"auth"`
	}
	cfg := struct {
		Auths map[string]authEntry `json:"auths"`
	}{
		Auths: map[string]authEntry{
			primarySvc: {
				Username: ro.Name,
				Password: ro.Password,
				Auth:     auth,
			},
		},
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal docker config: %w", err)
	}

	return base64.StdEncoding.EncodeToString(cfgJSON), nil
}
