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

package derived_status

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// TestBuildNodeGroupBlob_ChecksumGoldens renders real provider checksum templates over the
// elements the corpus produces and pins the result as a literal. The checksum names the CAPI
// machine template and the MCM MachineClass, both immutable: a checksum that moves renames them,
// which rolls the MachineDeployment and recreates every VM in it.
//
// The sibling pins in machineclass/ feed the templates a hand-written instanceClass; these ones
// start from buildNodeGroupBlob, so they also cover how the element carries the value there —
// the Go type of a number included, since toYaml renders float64 and int64 differently.
func TestBuildNodeGroupBlob_ChecksumGoldens(t *testing.T) {
	const (
		awsMCMTemplate     = "../../../../../../../../030-cloud-provider-aws/cloud-instance-manager/machine-class.checksum"
		yandexMCMTemplate  = "../../../../../../../../030-cloud-provider-yandex/cloud-instance-manager/machine-class.checksum"
		yandexCAPITemplate = "../../../../../../../../030-cloud-provider-yandex/capi/instance-class.checksum"
	)

	cases := []struct {
		name     string
		fixture  string
		template string
		want     string
	}{
		{
			name:     "yandex mcm",
			fixture:  "cloud-ephemeral-processed-full",
			template: yandexMCMTemplate,
			want:     "81b2646a77c8ff6a1a58dcbe2ca96c8faeef29def5a369fcfe84bba5be81ba5f",
		},
		{
			name:     "yandex capi",
			fixture:  "cloud-ephemeral-processed-full",
			template: yandexCAPITemplate,
			want:     "81b2646a77c8ff6a1a58dcbe2ca96c8faeef29def5a369fcfe84bba5be81ba5f",
		},
		{
			name:     "aws mcm",
			fixture:  "cloud-ephemeral-processed-empty-zones",
			template: awsMCMTemplate,
			want:     "4abe09d678acba9e38e29ee7926625af67e4e414ddf8246fb0368e7734ec7ab8",
		},
		{
			name:     "yandex mcm with fractional and zero numbers",
			fixture:  "numbers-and-quantities",
			template: yandexMCMTemplate,
			want:     "bb75a8b023691185dff0996ae8c529b60c7fe1069ecde719752e8b5ead766c00",
		},
		{
			name:     "aws mcm with empty map and list",
			fixture:  "numbers-and-quantities",
			template: awsMCMTemplate,
			want:     "aac113b58af656b614d46eb38fc205f611333e69a89c453f2e0148215dee5adc",
		},
	}

	byName := make(map[string]blobFixture, len(blobCorpus()))
	for _, fixture := range blobCorpus() {
		byName[fixture.name] = fixture
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture, ok := byName[tc.fixture]
			require.True(t, ok, "unknown corpus fixture %q", tc.fixture)

			tmpl, err := os.ReadFile(tc.template)
			require.NoError(t, err, "provider checksum template must exist")

			got, err := machineclass.RenderChecksum(tmpl, buildNodeGroupBlob(fixture.input, fixture.result), nil)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
