// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Hooks is meant to 'mark` a cluster of v1.55 as an 'old' one. This mark will be used in future releases for 'new' clusters
// by admission-policy-engine module to decide if default PodSecurityStandard profile should be set to Baseline by default.
// **REMOVE IN v1.56 release**
package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"

	"golang.org/x/mod/semver"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	milestone                       = "v1.55"
	admissionPolicyEngineAnnotation = "admission-policy-engine.deckhouse.io/pss-profile-milestone"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 25},
}, dependency.WithExternalDependencies(markSystemNamespace))

func markSystemNamespace(input *go_hook.HookInput, dc dependency.Container) error {
	deckhouseVersion := input.Values.Get("global.deckhouseVersion").String()
	if !semver.IsValid(deckhouseVersion) {
		input.LogEntry.Warnf("deckhouseVersion isn't valid semver: %s", deckhouseVersion)
		return nil
	}

	// check if deckhouse equals to v1.55.x
	if semver.Compare(semver.MajorMinor(deckhouseVersion), milestone) != 0 {
		return nil
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("couldn't get client: %v", err)
	}

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), d8Namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get %s namespace", d8Namespace)
	}

	if _, exist := ns.ObjectMeta.Annotations[admissionPolicyEngineAnnotation]; !exist {
		d8SystemPatch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					admissionPolicyEngineAnnotation: milestone,
				},
			},
		}
		input.PatchCollector.MergePatch(d8SystemPatch, "v1", "Namespace", "", d8Namespace)
	}

	return nil
}
