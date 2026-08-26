/*
Copyright 2021 Flant JSC

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

package template_tests

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func getDecodedSecretValue(s *object_store.KubeObject, path string) string {
	fullPath := fmt.Sprintf("data.%s", path)
	return decodeK8sObjField(s, fullPath)
}

func decodeK8sObjField(o *object_store.KubeObject, fullPath string) string {
	encodedVal := o.Field(fullPath).String()

	decodedArray, err := base64.StdEncoding.DecodeString(encodedVal)

	decodedVal := ""
	if err == nil {
		decodedVal = string(decodedArray)
	}

	return decodedVal
}

// assertKeepPolicyCovers binds the names helm renders to the shapes
// set_keep_policy_on_capi_resources selects Secrets by. The names are read off the
// rendered objects, so a template whose naming formula moved fails here instead of
// silently losing the annotation and being pruned during the handover.
func assertKeepPolicyCovers(taken, untouched []object_store.KubeObject) {
	for _, obj := range taken {
		name := obj.Field("metadata.name").String()
		Expect(name).NotTo(BeEmpty())
		Expect(hooks.IsBootstrapSecretName(name)).To(BeTrue(),
			"keep policy must cover the rendered secret %s", name)
	}
	for _, obj := range untouched {
		name := obj.Field("metadata.name").String()
		Expect(name).NotTo(BeEmpty())
		Expect(hooks.IsBootstrapSecretName(name)).To(BeFalse(),
			"keep policy must leave the rendered secret %s alone", name)
	}
}
