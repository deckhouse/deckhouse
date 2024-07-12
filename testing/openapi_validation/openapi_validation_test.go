//go:build validation
// +build validation

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

package openapi_validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationOpenAPI(t *testing.T) {
	apiFiles, err := GetOpenAPIYAMLFiles(deckhousePath)
	require.NoError(t, err)

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	for result := range resultC {
		assert.NoError(t, result.validationError, "File '%s' has invalid spec", strings.TrimPrefix(result.filePath, deckhousePath))
	}
}

// TestValidators test that validation hooks are working
func TestValidators(t *testing.T) {
	apiFiles := []string{deckhousePath + "testing/openapi_validation/openapi_testdata/values.yaml"}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	for res := range resultC {
		assert.Error(t, res.validationError)
		err, ok := res.validationError.(*multierror.Error)
		require.True(t, ok)
		require.Len(t, err.Errors, 6)

		// we can't guarantee order here, thats why test contains
		assert.Contains(t, res.validationError.Error(), "properties.https is invalid: must have no default value")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value 'disabled' must start with Capital letter")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Cert-Manager' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Some:Thing' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "Enum 'properties.https.properties.mode.enum' is invalid: value: 'Any.Thing' must be in CamelCase")
		assert.Contains(t, res.validationError.Error(), "properties.highAvailability is invalid: must have no default value")
	}
}

func TestCRDValidators(t *testing.T) {
	apiFiles := []string{deckhousePath + "testing/openapi_validation/openapi_testdata/crd.yaml"}

	filesC := make(chan fileValidation, len(apiFiles))
	resultC := RunOpenAPIValidator(filesC)

	for _, apiFile := range apiFiles {
		filesC <- fileValidation{
			filePath: apiFile,
		}
	}
	close(filesC)

	for res := range resultC {
		assert.Error(t, res.validationError)
		err, ok := res.validationError.(*multierror.Error)
		require.True(t, ok)
		require.Len(t, err.Errors, 1)

		// we can't guarantee order here, thats why test contains
		assert.Contains(t, res.validationError.Error(), "file validation error: wrong property")
	}
}

func TestModulesVersionsValidation(t *testing.T) {
	mv, err := modulesVersions(deckhousePath)
	require.NoError(t, err)
	for m, v := range mv {
		message := fmt.Sprintf("conversions version(%d) and spec version(%d) for module %s are not equal",
			v.conversionsVersion, v.specVersion, m)
		assert.Equal(t, true, v.conversionsVersion == v.specVersion, message)
	}
}
