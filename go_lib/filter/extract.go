// Copyright 2021 Flant JSC
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

package filter

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func KeyFromConfigMap(key string) func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
		var cm v1core.ConfigMap
		err := sdk.FromUnstructured(obj, &cm)
		if err != nil {
			return "", fmt.Errorf("from unstructured: %w", err)
		}

		val, ok := cm.Data[key]
		if !ok {
			return "", fmt.Errorf("no key %q in configmap %s", key, obj.GetName())
		}

		return val, nil
	}
}
