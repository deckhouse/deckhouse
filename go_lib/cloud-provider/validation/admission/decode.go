// Copyright 2026 Flant JSC
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

package admission

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

// DecodeModuleConfigObject decodes an admission ModuleConfig object.
func DecodeModuleConfigObject[S cpapi.ModuleSettingsObject](moduleName string, obj runtime.Object) (*cpapi.ModuleConfig[S], error) {
	objMap, err := runtimeObjectToMap(obj)
	if err != nil {
		return nil, fmt.Errorf("convert runtime object to map: %w", err)
	}

	moduleConfig, err := cpval.DecodeModuleConfig[S](moduleName, objMap)
	if err != nil {
		return nil, fmt.Errorf("decode ModuleConfig: %w", err)
	}

	return moduleConfig, nil
}

// DecodeNodeGroupObject decodes an admission NodeGroup object.
func DecodeNodeGroupObject(obj runtime.Object) (*cpapi.NodeGroup, error) {
	objMap, err := runtimeObjectToMap(obj)
	if err != nil {
		return nil, fmt.Errorf("convert runtime object to map: %w", err)
	}

	nodeGroup, err := cpval.DecodeNodeGroup(objMap)
	if err != nil {
		return nil, fmt.Errorf("decode NodeGroup: %w", err)
	}

	return nodeGroup, nil
}

// DecodeInstanceClassObject decodes an admission provider InstanceClass object.
// Prefer StateBuilderFactory.DecodeInstanceClass, which names the provider Kind on failure.
func DecodeInstanceClassObject[IC cpapi.InstanceClassObject](obj runtime.Object) (IC, error) {
	var absentClass IC

	objMap, err := runtimeObjectToMap(obj)
	if err != nil {
		return absentClass, fmt.Errorf("convert runtime object to map: %w", err)
	}

	instanceClass, err := cpval.DecodeInstanceClass[IC](objMap)
	if err != nil {
		return absentClass, err
	}

	return instanceClass, nil
}

// runtimeObjectToMap converts an admission object into an untyped map.
//
// A nil object yields a nil map rather than an error, so the decoders above report an absent
// resource: a webhook may hand over a missing old object, and the builder steps decide on their
// own whether an object is expected at all.
func runtimeObjectToMap(obj runtime.Object) (map[string]any, error) {
	if obj == nil {
		return nil, nil
	}

	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		return unstructuredObj.Object, nil
	}

	raw, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("marshal runtime object: %w", err)
	}

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("unmarshal runtime object: %w", err)
	}

	return object, nil
}
