/*
Copyright 2026 Flant JSC

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

package webhook

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// convertNodeUser ports the shell hook webhooks/conversion/node_user. The status default cannot be
// left to the CRD: an object stored in etcd as v1alpha1 is defaulted by the v1alpha1 schema, which
// has no status, and the v1 defaults are not applied after conversion.
func convertNodeUser(c *conversionContext) ([]byte, error) {
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(c.raw); err != nil {
		return nil, fmt.Errorf("unmarshal NodeUser %s: %w", c.srcVersion, err)
	}

	switch {
	case c.srcVersion == "deckhouse.io/v1alpha1" && c.desiredVersion == "deckhouse.io/v1":
		if obj.Object["status"] == nil {
			obj.Object["status"] = map[string]any{"errors": map[string]any{}}
		}
	case c.srcVersion == "deckhouse.io/v1" && c.desiredVersion == "deckhouse.io/v1alpha1":
		delete(obj.Object, "status")
	default:
		return nil, fmt.Errorf("unsupported NodeUser conversion from %s to %s", c.srcVersion, c.desiredVersion)
	}
	obj.SetAPIVersion(c.desiredVersion)

	return obj.MarshalJSON()
}
