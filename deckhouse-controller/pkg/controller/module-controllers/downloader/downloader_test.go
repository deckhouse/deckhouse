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

package downloader

import (
	"io"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestOpenapiInjection(t *testing.T) {
	source := `
x-extend:
  schema: config-values.yaml
type: object
properties:
  internal:
    type: object
    default: {}
    properties:
      pythonVersions:
        type: array
        default: []
        items:
          type: string
  registry:
    type: object
    description: "System field, overwritten by Deckhouse. Don't use"
`

	sourceModule := &v1alpha1.ModuleSource{}
	sourceModule.Spec.Registry.Repo = "test.deckhouse.io/foo/bar"
	sourceModule.Spec.Registry.DockerCFG = "dGVzdG1lCg=="
	sourceModule.Spec.Registry.Scheme = "http"
	sourceModule.Spec.Registry.CA = "someCA"

	data, err := mutateOpenapiSchema([]byte(source), sourceModule)
	require.NoError(t, err)

	assert.YAMLEq(t, `
type: object
x-extend:
  schema: config-values.yaml
properties:
  registry:
    type: object
    default: {}
    properties:
      base:
        type: string
        default: test.deckhouse.io/foo/bar
      dockercfg:
        type: string
        default: dGVzdG1lCg==
      scheme:
        type: string
        default: http
      ca:
        type: string
        default: someCA
  internal:
    default: {}
    properties:
      pythonVersions:
        default: []
        items:
          type: string
        type: array
    type: object
`, string(data))
}

func Test_checkModuleConfig(t *testing.T) {
	log.SetOutput(io.Discard)

	check := func(name string, failed bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("./testdata", name)
			err := checkModuleConfigFromFS(path)
			if err != nil && !failed {
				t.Fatalf("%s: unexpected error: %v", name, err)
			}
		})
	}

	check("module", false)
	check("module-not-valid", true)
	check("module-failed", true)
	check("module-values-failed", true)
}
