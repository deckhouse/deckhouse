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

package image_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryConfig_ListTags(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "image")
	err := createImageFiles(basePath)
	require.NoError(t, err)

	type fields struct {
		registryPath string
		authConfig   *types.DockerAuthConfig
	}
	type args struct {
		ctx  context.Context
		opts []image.ListOption
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		want        []string
		wantInitErr error
		wantErr     error
	}{
		{
			name: "list deckhouse skopeo",
			fields: fields{
				registryPath: "docker://registry.deckhouse.io/deckhouse/tools/skopeo/",
			},
			args: args{
				ctx: context.Background(),
			},
			want: []string{"v1.11.2"},
		},
		{
			name: "list directory",
			fields: fields{
				registryPath: fmt.Sprintf("dir:%s", basePath),
			},
			args: args{
				ctx: context.Background(),
			},
			wantErr: image.ErrDirNotImplemented,
		},
		{
			name: "list file",
			fields: fields{
				registryPath: fmt.Sprintf("file:%s", util.AddTarGzExt(basePath)),
			},
			args: args{
				ctx: context.Background(),
			},
			want: []string{"image-1", "image-2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := image.NewRegistry(tt.fields.registryPath, tt.fields.authConfig, true)
			require.ErrorIs(t, err, tt.wantInitErr)

			got, err := r.ListTags(tt.args.ctx, tt.args.opts...)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func createImageFiles(basePath string) error {
	for i := 1; i < 3; i++ {
		imgDir := fmt.Sprintf("%s/image-%d", basePath, i)
		if err := createImage(imgDir); err != nil {
			return err
		}
	}
	return util.CompressDir(basePath, true)
}

func createImage(imgDir string) error {
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return err
	}

	for _, filename := range []string{"manifest.json", "version"} {
		if err := createFile(imgDir, filename); err != nil {
			return err
		}
	}

	return util.CompressDir(imgDir, true)
}
func createFile(imgDir, filename string) error {
	f, err := os.Create(filepath.Join(imgDir, filename))
	if err != nil {
		return err
	}
	return f.Close()
}
