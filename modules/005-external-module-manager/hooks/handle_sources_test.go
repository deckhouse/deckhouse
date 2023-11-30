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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
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

	sourceModule := v1alpha1.ModuleSource{}
	sourceModule.Spec.Registry.Repo = "test.deckhouse.io/foo/bar"
	sourceModule.Spec.Registry.DockerCFG = "dGVzdG1lCg=="

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

func TestDownloadImage(t *testing.T) {
	t.Skip("For manual run and check images")

	dockerCfg := ""
	repo := ""
	tag := ""

	opts := make([]cr.Option, 0)
	opts = append(opts, cr.WithAuth(dockerCfg))

	c, err := cr.NewClient(repo, opts...)
	require.NoError(t, err)

	img, err := c.Image(tag)
	require.NoError(t, err)

	err = os.MkdirAll("/tmp/module", 0777)
	require.NoError(t, err)

	err = copyModuleToFS("/tmp/module", img)
	require.NoError(t, err)
}
