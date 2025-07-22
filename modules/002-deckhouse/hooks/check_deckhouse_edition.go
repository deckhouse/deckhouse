package hooks

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"

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
			Name:       "module-configs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			FilterFunc: applyModuleConfigFilter,
		},
	},
}, handleModuleConfig)
var reEditionFromPath = regexp.MustCompile(`^/deckhouse/(.*)$`)

func applyModuleConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	edition, _, _ := unstructured.NestedString(obj.Object, "spec", "settings", "licence", "edition")
	return edition, nil
}

func validateEdition(edition string) bool {
	return slices.Contains([]string{"ce", "be", "fe", "ee", "se", "se-plus"}, edition)
}

func handleModuleConfig(input *go_hook.HookInput) error {
	input.Logger.Info("check_deckhouse_edition hook handled")

	// check moduleConfig spec.settings.licence.edition
	moduleConfigs := input.NewSnapshots.Get("module-configs")
	mcSlice := set.NewFromSnapshot(moduleConfigs).Slice()
	for _, mc := range mcSlice {
		if validateEdition(mc) {
			input.Logger.Info("check_deckhouse_edition", slog.String("moduleConfig.edition", mc))
			return nil
		}
	}

	// check values.global.deckhouseEdition
	edition, ok := input.Values.GetOk("global.deckhouseEdition")
	input.Logger.Info("check_deckhouse_edition", slog.String("global.deckhouseEdition", edition.String()))
	if ok && validateEdition(edition.String()) {
		input.Logger.Info("")
		return nil
	}

	// check values.global.registry.edition
	registryAddress, ok := input.Values.GetOk("global.modulesImages.registry.address")
	input.Logger.Info("check_deckhouse_edition", slog.String("global.modulesImages.registry.address", registryAddress.String()))
	if ok && registryAddress.String() == "registry.deckhouse.io" {
		registryPath, ok := input.Values.GetOk("global.modulesImages.registry.path")
		if !ok {
			input.Logger.Warn("check_deckhouse_edition global.modulesImages.registry.path not set")
			return nil
		}
		input.Logger.Info("check_deckhouse_edition", slog.String("global.modulesImages.registry.path", registryPath.String()))
		reResult := reEditionFromPath.FindStringSubmatch(registryPath.String())
		input.Logger.Info("check_deckhouse_edition", slog.Any("reResult", reResult))
		if len(reResult) > 1 && validateEdition(reResult[1]) {
			input.Logger.Info("check_deckhouse_edition", slog.String("reResult[1]", reResult[1]))
			return nil
		}

		input.Logger.Warn("check_deckhouse_edition global.modulesImages.registry.path does not match edition regex")
	}

	// if we reach this point, it means no edition was found
	input.MetricsCollector.Set("deckhouse_edition_not_found", 1.0, nil)
	return fmt.Errorf("please set the deckhouse edition in ModuleConfig")
}
