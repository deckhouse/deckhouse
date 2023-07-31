// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"fmt"
	"testing"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageConfig_imageReference(t *testing.T) {
	basePath := t.TempDir()

	type fields struct {
		tag             string
		digest          string
		additionalPaths []string
		registry        *RegistryConfig
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr error
	}{
		{
			name: "file transport",
			fields: fields{
				tag:             "test-tag",
				digest:          "test-digest",
				additionalPaths: []string{"testPath"},
				registry:        MustNewRegistry(fmt.Sprintf("file:%s", basePath), nil, false),
			},
			want: fmt.Sprintf("file:%s/testPath/test-tag@test-digest", basePath),
		},
		{
			name: "dir transport",
			fields: fields{
				tag:             "test-tag",
				digest:          "test-digest",
				additionalPaths: []string{"testPath"},
				registry:        MustNewRegistry(fmt.Sprintf("dir:%s", basePath), nil, false),
			},
			want: fmt.Sprintf("dir:%s/testPath/test-tag@test-digest", basePath),
		},
		{
			name: "docker transport with tag and digest",
			fields: fields{
				tag:             "test-tag",
				digest:          "sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4",
				additionalPaths: []string{"testpath"},
				registry:        MustNewRegistry("docker://test.com/test", nil, false),
			},
			want: "docker://test.com/test/testpath@sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4",
		},

		{
			name: "docker transport with tag",
			fields: fields{
				tag:             "test-tag",
				additionalPaths: []string{"testpath"},
				registry:        MustNewRegistry("docker://test.com/test", nil, false),
			},
			want: "docker://test.com/test/testpath:test-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := NewImageConfig(tt.fields.registry, tt.fields.tag, tt.fields.digest, tt.fields.additionalPaths...)
			got, err := i.imageReference(false, true)
			require.ErrorIs(t, err, tt.wantErr)

			want := mustNewImageRef(t, tt.want)
			assert.Equal(t, got, want)
		})
	}
}

func mustNewImageRef(t *testing.T, image string) types.ImageReference {
	imgRef, err := alltransports.ParseImageName(image)
	require.NoError(t, err)
	return imgRef
}
