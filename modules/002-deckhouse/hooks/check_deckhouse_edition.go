/*
Copyright 2025 Flant JSC

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
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "moduleconfigs",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"}, // only deckhouse module config
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
}, handleModuleConfigWrap())

var reEditionFromPath = regexp.MustCompile(`^/deckhouse/(.+)$`)
var allExpectedEditions = []string{"ce", "be", "ee", "se", "se-plus", "fe"}

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	edition, _, _ := unstructured.NestedString(obj.Object, "spec", "settings", "license", "edition")
	return edition, nil // snapshot is a string with edition
}

func validateEdition(edition string) bool {
	edition = strings.ToLower(edition)
	return slices.Contains(allExpectedEditions, edition)
}

func handleModuleConfigWrap() func(_ context.Context, _ *go_hook.HookInput) error {
	// skip check for dev
	versionContent, readErr := os.ReadFile("/deckhouse/version")

	if readErr == nil {
		version := strings.TrimSuffix(string(versionContent), "\n")
		if version == "dev" {
			return func(_ context.Context, _ *go_hook.HookInput) error { return nil }
		}
	}

	return func(_ context.Context, input *go_hook.HookInput) error {
		if readErr != nil {
			input.Logger.Warn("can't read deckhouse version file", log.Err(readErr))
		}

		// set metrics on hook result
		var found bool
		defer func(found *bool) {
			var value float64
			if !*found {
				value = 1.0
			}
			input.MetricsCollector.Set("d8_edition_not_found", value, nil)
		}(&found)

		// check values.global.deckhouseEdition
		edition, ok := input.Values.GetOk("global.deckhouseEdition")
		if ok && validateEdition(edition.String()) {
			found = true
			return nil
		}

		// check values.global.registry
		registryAddress, ok := input.Values.GetOk("global.modulesImages.registry.address")
		if ok && registryAddress.String() == "registry.deckhouse.io" {
			// if prod registry, check path
			registryPath, ok := input.Values.GetOk("global.modulesImages.registry.path")
			if !ok {
				input.Logger.Warn("global value global.modulesImages.registry.path not set")
				return nil
			}

			// regex to extract edition from path
			// e.g. /deckhouse/ce, /deckhouse/be, /deckhouse/ee, /deckhouse/se, /deckhouse/se-plus
			reResult := reEditionFromPath.FindStringSubmatch(registryPath.String())
			if len(reResult) > 0 && validateEdition(reResult[1]) {
				found = true
				return nil
			}

			input.Logger.Warn("global value global.modulesImages.registry.path does not match edition regex")
		}

		// check moduleConfig spec.settings.licence.edition
		moduleEditions := input.Snapshots.Get("moduleconfigs") // snapshot is a string with edition
		for _, moduleEditionSnap := range moduleEditions {
			moduleEdition := moduleEditionSnap.String()
			if validateEdition(moduleEdition) {
				found = true
				return nil
			}
		}

		// if we reach this point, it means no edition was found
		input.Logger.Warn("deckhouse edition not found")
		return nil
	}
}
