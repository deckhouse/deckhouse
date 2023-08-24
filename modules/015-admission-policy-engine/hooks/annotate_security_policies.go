/*
Copyright 2022 Flant JSC

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
	"fmt"
	"time"

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// hook for setting CR statuses
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/admission-policy-engine/security_policies",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, annotateSP)

var processedStatus = func(filterFunc func(*unstructured.Unstructured) (go_hook.FilterResult, error)) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		objCopy := obj.DeepCopy()
		filteredObj, err := filterFunc(objCopy)
		if err != nil {
			return nil, fmt.Errorf("cannot apply filterFunc to object: %v", err)
		}

		objBytes, err := json.Marshal(filteredObj)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal filtered object: %v", err)
		}

		objCheckSum := utils_checksum.CalculateChecksum(string(objBytes))

		observedCheckSum, found, err := unstructured.NestedString(objCopy.Object, "status", "deckhouse", "observed", "checkSum")
		if err != nil {
			return nil, fmt.Errorf("cannot get observed checksum status field: %v", err)
		}

		if !found || objCheckSum != observedCheckSum {
			if err := unstructured.SetNestedField(objCopy.Object, "False", "status", "deckhouse", "synced"); err != nil {
				return nil, fmt.Errorf("cannot set synced status field: %v", err)
			}
		} else {
			if err := unstructured.SetNestedField(objCopy.Object, "True", "status", "deckhouse", "synced"); err != nil {
				return nil, fmt.Errorf("cannot set synced status field: %v", err)
			}
		}

		if err := unstructured.SetNestedStringMap(objCopy.Object, map[string]string{"lastTimestamp": time.Now().Format(time.RFC3339), "checkSum": objCheckSum}, "status", "deckhouse", "processed"); err != nil {
			return nil, fmt.Errorf("cannot set processed status field: %v", err)
		}
		return objCopy, nil
	}
}

func annotateSP(input *go_hook.HookInput) error {
	securityPolicies := make([]securityPolicy, 0)

	err := json.Unmarshal([]byte(input.Values.Get("admissionPolicyEngine.internal.securityPolicies").String()), &securityPolicies)
	if err != nil {
		return fmt.Errorf("cannot unmarshal values: %v", err)
	}

	for _, sp := range securityPolicies {
		input.PatchCollector.Filter(processedStatus(filterSP), "deckhouse.io/v1alpha1", "securitypolicy", "", sp.Metadata.Name, object_patch.WithSubresource("/status"))
	}

	return nil
}
