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

package values_validation

import (
	"testing"

	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/stretchr/testify/require"
)

func Test_ValidateValues_check_for_known_go_1_16_problem(t *testing.T) {
	const message = `test should not fail with error 'must be of type string: "null"'.
 There is a problem with Go 1.16 and go-openapi, see https://github.com/go-openapi/validate/issues/137.
 go-openapi should be updated in addon-operator to use Go 1.16 in deckhouse.`
	const values = `
{"nodeManager":{
  "internal":{
    "manualRolloutID":""
  }
}}`
	const schema = `
type: object
properties:
  internal:
    type: object
    properties:
      manualRolloutID:
        type: string
`

	schemaStorage, err := validation.NewSchemaStorage([]byte{}, []byte(schema))
	require.NoError(t, err, "should load schema")

	valuesValidator := &ValuesValidator{
		ModuleSchemaStorages: map[string]*validation.SchemaStorage{
			"nodeManager": schemaStorage,
		},
	}

	// Validate empty string
	err = valuesValidator.ValidateJSONValues("nodeManager", []byte(values), false)
	require.NoError(t, err, message)
}
