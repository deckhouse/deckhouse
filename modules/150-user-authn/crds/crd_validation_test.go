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

package crds_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
	"sigs.k8s.io/yaml"
)

// The API server validates a CRD before it stores it: the structural schema, and every CEL rule with its estimated
// cost against the budget of a rule and of the whole object. A rule over a list or a string without maxItems or
// maxLength is estimated for the largest object the API server takes and may exceed the budget, and then the CRD is
// refused and the module does not install. The tests of the rules themselves compile them without the budget, so this
// test runs the validation of the API server on every CRD of the module.
func TestCRDsPassTheValidationOfTheAPIServer(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)
	checked := 0
	for _, file := range files {
		if strings.HasPrefix(file, "doc-ru-") {
			continue
		}
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(file)
			require.NoError(t, err)
			var crdV1 apiextensionsv1.CustomResourceDefinition
			require.NoError(t, yaml.Unmarshal(data, &crdV1))
			require.Equal(t, "CustomResourceDefinition", crdV1.Kind, file)
			apiextensionsv1.SetObjectDefaults_CustomResourceDefinition(&crdV1)

			var crd apiextensions.CustomResourceDefinition
			require.NoError(t, apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&crdV1, &crd, nil))
			for _, version := range crd.Spec.Versions {
				if version.Storage {
					crd.Status.StoredVersions = []string{version.Name}
				}
			}

			errs := validation.ValidateCustomResourceDefinition(t.Context(), &crd)
			require.Empty(t, errs.ToAggregate(), "the API server refuses %s", file)
		})
		checked++
	}
	require.Positive(t, checked)
}
