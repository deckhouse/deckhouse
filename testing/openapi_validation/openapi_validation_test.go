/*
Copyright 2021 Flant CJSC

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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIValidation(t *testing.T) {
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
		assert.NoError(t, result.enumErr, "File '%s' has invalid spec", strings.TrimPrefix(result.filePath, deckhousePath))
	}
}
