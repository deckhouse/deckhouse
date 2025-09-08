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
	"context"
	"fmt"

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
)

// hook for setting CR statuses
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/admission-policy-engine/security_policies",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, updateSpStatus)

func updateSpStatus(_ context.Context, input *go_hook.HookInput) error {
	securityPolicies := make([]securityPolicy, 0)

	// get SPs' names
	err := json.Unmarshal([]byte(input.Values.Get("admissionPolicyEngine.internal.securityPolicies").String()), &securityPolicies)
	if err != nil {
		return fmt.Errorf("cannot unmarshal values: %v", err)
	}

	// update SPs' statuses
	for _, sp := range securityPolicies {
		input.PatchCollector.PatchWithMutatingFunc(set_cr_statuses.SetProcessedStatus(filterSP), "deckhouse.io/v1alpha1", "securitypolicy", "", sp.Metadata.Name, object_patch.WithSubresource("/status"), object_patch.WithIgnoreHookError())
	}

	return nil
}
