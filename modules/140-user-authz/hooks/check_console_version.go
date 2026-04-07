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
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	consoleModuleName         = "console"
	consoleMinVersionRequired = "1.44.0"
)

var consoleMinVersion = semver.MustParse(consoleMinVersionRequired)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "console_module",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "Module",
			NameSelector:                 &types.NameSelector{MatchNames: []string{consoleModuleName}},
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyConsoleModuleFilter,
		},
	},
}, handleConsoleVersionCheck)

type consoleModuleInfo struct {
	Version string
}

func applyConsoleModuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "properties", "version")
	if err != nil {
		return nil, err
	}
	return consoleModuleInfo{Version: version}, nil
}

func handleConsoleVersionCheck(_ context.Context, input *go_hook.HookInput) error {
	snap := input.Snapshots.Get("console_module")

	// If console module is not installed, no legacy compat needed
	if len(snap) == 0 {
		input.Values.Set("userAuthz.internal.consoleLegacyCompat", false)
		input.Values.Remove("userAuthz.internal.consoleVersion")
		return nil
	}

	for info, err := range sdkobjectpatch.SnapshotIter[consoleModuleInfo](snap) {
		if err != nil {
			return err
		}

		if info.Version == "" {
			// Module exists but no version info, assume no compat needed
			input.Values.Set("userAuthz.internal.consoleLegacyCompat", false)
			input.Values.Remove("userAuthz.internal.consoleVersion")
			return nil
		}

		input.Values.Set("userAuthz.internal.consoleVersion", info.Version)

		// Parse version (strip leading 'v' if present for semver parsing)
		versionStr := strings.TrimPrefix(info.Version, "v")
		consoleVersion, err := semver.NewVersion(versionStr)
		if err != nil {
			// Failed to parse version, assume no compat needed
			input.Values.Set("userAuthz.internal.consoleLegacyCompat", false)
			return nil
		}

		// If console version < minimum required, enable legacy compat mode
		legacyCompat := consoleVersion.LessThan(consoleMinVersion)
		input.Values.Set("userAuthz.internal.consoleLegacyCompat", legacyCompat)

		// Only process first module (there should be only one)
		return nil
	}

	// No modules in snapshot
	input.Values.Set("userAuthz.internal.consoleLegacyCompat", false)
	input.Values.Remove("userAuthz.internal.consoleVersion")
	return nil
}
