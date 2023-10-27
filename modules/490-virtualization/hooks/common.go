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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	initValuesString       = `{"virtualization":{"internal":{},"vmCIDRs":["10.10.10.0/24"]},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{"virtualization":{"vmCIDRs":["10.10.10.0/24"]}}`
	gv                     = "deckhouse.io/v1alpha1"
)

func applyCRDExistenseFilter(_ *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return true, nil
}
