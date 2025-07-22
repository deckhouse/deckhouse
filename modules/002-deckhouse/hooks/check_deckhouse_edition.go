/*
Copyright 2023 Flant JSC

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
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/flow-schema",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "moduleconfigs",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "moduleconfigs",
			Crontab: "*/1 * * * *", // every minute
		},
	},
}, handleModuleConfig)
var reEditionFromPath = regexp.MustCompile(`^/deckhouse/(.+)$`)

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	edition, _, _ := unstructured.NestedString(obj.Object, "spec", "settings", "licence", "edition")
	return edition, nil
}

func validateEdition(edition string) bool {
	edition = strings.ToLower(edition)
	return slices.Contains([]string{"ce", "be", "fe", "ee", "se", "se-plus"}, edition)
}

func handleModuleConfig(input *go_hook.HookInput) error {
	input.Logger.Info("--- === hook handled === ---")

	// check moduleConfig spec.settings.licence.edition
	moduleConfigs := input.NewSnapshots.Get("moduleconfigs")
	input.Logger.Info("moduleConfigs length", slog.Int("length", len(moduleConfigs)))
	mcSlice := set.NewFromSnapshot(moduleConfigs).Slice()
	input.Logger.Info("mcSlice length", slog.Int("length", len(mcSlice)))
	for _, mc := range mcSlice {
		input.Logger.Info("iterate with mc", slog.String("mc", mc))
		if validateEdition(mc) {
			input.Logger.Info("mc validated", slog.String("mc", mc))
			// return nil
			return errors.New("TEST")
		}
	}

	// check values.global.deckhouseEdition
	edition, ok := input.Values.GetOk("global.deckhouseEdition")
	input.Logger.Info("trying to get edition from values", slog.String("global.deckhouseEdition", edition.String()), slog.Bool("ok", ok))
	if ok && validateEdition(edition.String()) {
		input.Logger.Info("edition validated", slog.String("edition", edition.String()))
		// return nil
		return errors.New("TEST")
	}

	// check values.global.registry.edition
	registryAddress, ok := input.Values.GetOk("global.modulesImages.registry.address")
	input.Logger.Info("trying to get edition from registry path", slog.String("global.modulesImages.registry.address", registryAddress.String()))
	if ok && registryAddress.String() == "registry.deckhouse.io" {
		// if prod registry, check path
		registryPath, ok := input.Values.GetOk("global.modulesImages.registry.path")
		if !ok {
			input.Logger.Warn("Global value global.modulesImages.registry.path not set")
			// return nil
			return errors.New("TEST")
		}
		input.Logger.Info("trying to get edition from registry path", slog.String("global.modulesImages.registry.path", registryPath.String()))

		// regex to extract edition from path
		// e.g. /deckhouse/ce, /deckhouse/be, /deckhouse/fe, /deckhouse/ee, /deckhouse/se, /deckhouse/se-plus
		reResult := reEditionFromPath.FindStringSubmatch(registryPath.String())
		input.Logger.Info("regex result", slog.Any("reResult", reResult))
		if len(reResult) > 0 && validateEdition(reResult[1]) {
			input.Logger.Info("edition validated", slog.String("reResult[1]", reResult[1]))
			// return nil
			return errors.New("TEST")
		}

		input.Logger.Warn("check_deckhouse_edition global.modulesImages.registry.path does not match edition regex")
	}

	// if we reach this point, it means no edition was found
	input.MetricsCollector.Set("deckhouse_edition_not_found", 1.0, nil)
	return errors.New("TEST")
}
