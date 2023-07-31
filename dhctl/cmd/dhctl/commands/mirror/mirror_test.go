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

package mirror

import (
	"os"
	"syscall"
	"testing"

	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_deckhouseEdition(t *testing.T) {
	tests := []struct {
		name          string
		want          string
		editionInFile string
		createFile    bool
		wantErr       error
	}{
		{
			name:          "not EE or FE edition",
			editionInFile: "ce",
			createFile:    true,
			wantErr:       ErrNotEE,
		},
		{
			name:          "FE edition",
			editionInFile: "fe",
			createFile:    true,
			want:          "fe",
		},
		{
			name:          "EE edition",
			editionInFile: "ee",
			createFile:    true,
			want:          "ee",
		},
		{
			name:    "no edition file",
			wantErr: (syscall.Errno)(2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				err := os.WriteFile("/deckhouse/edition", []byte(tt.editionInFile), 0o755)
				require.NoError(t, err)
				defer os.Remove("/deckhouse/edition")
			}

			got, err := deckhouseEdition()
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_deckhouseRegistry(t *testing.T) {
	type args struct {
		deckhouseRegistry string
		edtiton           string
		licenseToken      string
	}
	tests := []struct {
		name    string
		args    args
		want    *image.RegistryConfig
		wantErr error
	}{
		{
			name: "docker registry with license",
			args: args{
				deckhouseRegistry: "docker://registry.deckhouse.io/deckhouse",
				edtiton:           "ee",
				licenseToken:      "token",
			},
			want: image.MustNewRegistry("docker://registry.deckhouse.io/deckhouse/ee", &types.DockerAuthConfig{Username: "license-token", Password: "token"}, true),
		},
		{
			name: "docker registry without license",
			args: args{
				deckhouseRegistry: "docker://registry.deckhouse.io/deckhouse",
				edtiton:           "ee",
			},
			wantErr: ErrNoLicense,
		},
		{
			name: "file registry",
			args: args{
				deckhouseRegistry: "file:versions/fixtures/deckhouse-registry.tar.gz",
				edtiton:           "ee",
			},
			want: image.MustNewRegistry("file:versions/fixtures/deckhouse-registry.tar.gz", nil, false),
		},
		{
			name: "bad transport registry",
			args: args{
				deckhouseRegistry: "docker-archive://tar.gz",
				edtiton:           "ee",
			},
			wantErr: image.ErrNoSuchRegistryTransport,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deckhouseRegistry(tt.args.deckhouseRegistry, tt.args.edtiton, tt.args.licenseToken)
			if got != nil {
				defer got.Close()
			}
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_registryAuth(t *testing.T) {
	type args struct {
		username string
		password string
	}
	tests := []struct {
		name string
		args args
		want *types.DockerAuthConfig
	}{
		{
			name: "username and password set",
			args: args{
				username: "user",
				password: "password",
			},
			want: &types.DockerAuthConfig{
				Username: "user",
				Password: "password",
			},
		},

		{
			name: "username set and password not set",
			args: args{
				username: "user",
			},
			want: nil,
		},

		{
			name: "username not set and password set",
			args: args{
				password: "password",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registryAuth(tt.args.username, tt.args.password)
			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_destinationImage(t *testing.T) {
	type args struct {
		destRegistry *image.RegistryConfig
		srcImage     *image.ImageConfig
	}
	tests := []struct {
		name string
		args args
		want *image.ImageConfig
	}{
		{
			name: "image with tag and digest for file",
			args: args{
				destRegistry: image.MustNewRegistry("file:result.tar.gz", nil, false),
				srcImage:     image.NewImageConfig(image.MustNewRegistry("docker://registry.test.com", nil, true), "test-tag", "test-digest", "additional-path"),
			},
			want: image.NewImageConfig(image.MustNewRegistry("file:result.tar.gz", nil, false), "test-tag", "test-digest", "additional-path"),
		},
		{
			name: "image with tag and digest for docker",
			args: args{
				destRegistry: image.MustNewRegistry("docker://registry.result.com", nil, false),
				srcImage:     image.NewImageConfig(image.MustNewRegistry("docker://registry.test.com", nil, true), "test-tag", "test-digest", "additional-path"),
			},
			want: image.NewImageConfig(image.MustNewRegistry("docker://registry.result.com", nil, false), "test-tag", "", "additional-path"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := destinationImage(tt.args.destRegistry, tt.args.srcImage)
			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_sourceImage(t *testing.T) {
	type args struct {
		srcImage *image.ImageConfig
	}
	tests := []struct {
		name string
		args args
		want *image.ImageConfig
	}{
		{
			name: "image with tag and digest docker",
			args: args{
				srcImage: image.NewImageConfig(image.MustNewRegistry("docker://registry.test.com", nil, true), "test-tag", "test-digest", "additional-path"),
			},
			want: image.NewImageConfig(image.MustNewRegistry("docker://registry.test.com", nil, true), "", "test-digest", "additional-path"),
		},
		{
			name: "image with tag and digest file",
			args: args{
				srcImage: image.NewImageConfig(image.MustNewRegistry("file:test.tar.gz", nil, false), "test-tag", "test-digest", "additional-path"),
			},
			want: image.NewImageConfig(image.MustNewRegistry("file:test.tar.gz", nil, false), "test-tag", "test-digest", "additional-path"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sourceImage(tt.args.srcImage)
			assert.Equal(t, got, tt.want)
		})
	}
}
