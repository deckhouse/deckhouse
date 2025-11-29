package hooks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const customConfigKey = "custom"
const customNamesPath = "nodeManager.internal.customMIGNames"

// Pre-Helm hook to compute resolved MIG config names for custom partitions.
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue:        "/modules/node-manager",
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "nodegroups",
				ApiVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
				FilterFunc: filterGPUSpec,
			},
		},
	},
	setResolvedMIGNames)

func setResolvedMIGNames(_ context.Context, input *go_hook.HookInput) error {
	ngs := input.Snapshots.Get("nodegroups")
	if len(ngs) == 0 {
		return nil
	}

	if err := ensureCustomNamesPath(input); err != nil {
		return err
	}

	original := map[string]string{}
	for k, v := range input.Values.Get(customNamesPath).Map() {
		original[k] = v.String()
	}

	customNames := map[string]string{}

	for _, ngSnapshot := range ngs {
		var ng nodeGroupInfo
		if err := ngSnapshot.UnmarshalTo(&ng); err != nil {
			return err
		}
		if ng.MIGConfig == nil || *ng.MIGConfig != customConfigKey {
			continue
		}
		if len(ng.CustomConfigs) == 0 {
			continue
		}

		name := resolveCustomMIGConfigName(ng.Name, ng.CustomConfigs)
		if name == "" {
			return fmt.Errorf("cannot resolve custom MIG config name for nodegroup %s", ng.Name)
		}
		customNames[ng.Name] = name
	}

	if reflect.DeepEqual(original, customNames) {
		return nil
	}
	input.Values.Set(customNamesPath, customNames)
	return nil
}

// ensureCustomNamesPath makes sure nodeManager.internal.customMIGNames exists to allow setting values in a clean state.
func ensureCustomNamesPath(input *go_hook.HookInput) error {
	if !input.Values.Exists("nodeManager") {
		input.Values.Set("nodeManager", map[string]interface{}{})
	}
	if !input.Values.Exists("nodeManager.internal") {
		input.Values.Set("nodeManager.internal", map[string]interface{}{})
	}
	if !input.Values.Exists(customNamesPath) {
		input.Values.Set(customNamesPath, map[string]interface{}{})
	}
	return nil
}

func resolveCustomMIGConfigName(ngName string, cfg []ngv1.MigCustomConfig) string {
	if len(cfg) == 0 {
		return ""
	}
	norm := normalizeCustomConfigs(cfg)
	hash := hashCustomConfigs(norm)
	if hash == "" {
		return ""
	}
	namePart := trimNameWithHash(ngName)
	return fmt.Sprintf("custom-%s-%s", namePart, hash)
}

func hashCustomConfigs(cfg []ngv1.MigCustomConfig) string {
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

const (
	maxLabelLen            = 63
	customPrefix           = "custom-"
	customHashLen          = 8
	customDelimiter        = "-"
	customNameHashReserve  = customHashLen + len(customDelimiter) // 9
	customNameAvailableLen = maxLabelLen - len(customPrefix) - len(customDelimiter) - customHashLen
)

func trimNameWithHash(name string) string {
	if len(name) <= customNameAvailableLen {
		return name
	}

	nameHash := hashString(name)
	prefixLen := customNameAvailableLen - customNameHashReserve
	if prefixLen < 1 {
		prefixLen = 1
	}
	if prefixLen > len(name) {
		prefixLen = len(name)
	}
	return fmt.Sprintf("%s-%s", name[:prefixLen], nameHash)
}

func hashString(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])[:customHashLen]
}

// normalizeCustomConfigs sorts configs by GPU index and slices by profile, and defaults count to 1.
func normalizeCustomConfigs(cfg []ngv1.MigCustomConfig) []ngv1.MigCustomConfig {
	out := make([]ngv1.MigCustomConfig, len(cfg))
	copy(out, cfg)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index < out[j].Index
	})

	for i := range out {
		if len(out[i].Slices) == 0 {
			continue
		}
		sort.Slice(out[i].Slices, func(a, b int) bool {
			return out[i].Slices[a].Profile < out[i].Slices[b].Profile
		})
		for j := range out[i].Slices {
			if out[i].Slices[j].Count == nil {
				val := int32(1)
				out[i].Slices[j].Count = &val
			}
		}
	}
	return out
}
