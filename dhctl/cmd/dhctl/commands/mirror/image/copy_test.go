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
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/stretchr/testify/require"
)

func TestCopyImage(t *testing.T) {
	if os.Getenv("D8_DHCTL_COPY_REGISTRY_VALIDATE") != "yes" {
		t.Skip("Do not run this on CI")
	}

	deckhouseRegistry, err := image.NewRegistry("docker://registry.deckhouse.io/deckhouse/ce/", nil)
	require.NoError(t, err)

	localFile, err := image.NewRegistry("file:"+filepath.Join(t.TempDir(), "file.tar.gz"), nil)
	require.NoError(t, err)

	localDir, err := image.NewRegistry("dir:"+filepath.Join(t.TempDir(), "dir"), nil)
	require.NoError(t, err)

	policyContext, err := image.NewPolicyContext()
	require.NoError(t, err)
	defer policyContext.Destroy()

	type args struct {
		ctx  context.Context
		src  *image.ImageConfig
		dest *image.ImageConfig
		opts []image.CopyOption
	}
	tests := []struct {
		name      string
		args      args
		wantErr   error
		checkDir  string
		checkFile string
	}{
		{
			name: "dryRun copy from deckhouse registry to deckhouse registry by sha",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "test-tag", "sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4"),
				dest: image.NewImageConfig(deckhouseRegistry, "copy-test-sha", ""),
				opts: []image.CopyOption{image.WithOutput(io.Discard), image.WithDryRun()},
			},
		},

		{
			name: "copy release-channel:alpha from deckhouse registry to local file by tag",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "alpha", "", "release-channel"),
				dest: image.NewImageConfig(localFile, "alpha", "", "release-channel"),
				opts: []image.CopyOption{image.WithOutput(io.Discard)},
			},
			checkFile: filepath.Join(localFile.Path(), "release-channel", "alpha.tar.gz"),
		},

		{
			name: "copy from deckhouse registry to local file by bad tag and sha",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "test-tag", "sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4"),
				dest: image.NewImageConfig(localFile, "copy-test-sha", ""),
				opts: []image.CopyOption{image.WithOutput(io.Discard)},
			},
			checkFile: filepath.Join(localFile.Path(), "copy-test-sha.tar.gz"),
		},

		{
			name: "copy from deckhouse registry to local dir by bad tag and sha",
			args: args{
				ctx:  context.Background(),
				src:  image.NewImageConfig(deckhouseRegistry, "test-tag", "sha256:79ecc9578e5d18a524f5fecc9e5eb82231191d4deafd27e51bed212f9da336d4"),
				dest: image.NewImageConfig(localDir, "copy-test-sha", ""),
				opts: []image.CopyOption{image.WithOutput(io.Discard)},
			},
			checkDir: filepath.Join(localDir.Path(), "copy-test-sha"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := image.CopyImage(tt.args.ctx, tt.args.src, tt.args.dest, policyContext, tt.args.opts...)
			require.ErrorIs(t, err, tt.wantErr)

			if tt.checkFile != "" {
				err := testCheckCopyFile(tt.checkFile)
				require.NoError(t, err)
			}

			if tt.checkDir != "" {
				err := testCheckCopyDir(tt.checkDir)
				require.NoError(t, err)
			}
		})
	}
}

func testCheckCopyDir(checkDir string) error {
	dirInfo, err := os.Stat(checkDir)
	if err != nil {
		return fmt.Errorf("CopyImage() error = path error for dir %s: %v", checkDir, err)
	}

	if !dirInfo.IsDir() {
		return fmt.Errorf("CopyImage() error = path is not a dir: %v", checkDir)
	}

	for _, f := range []string{"version", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(checkDir, f)); err != nil {
			return fmt.Errorf("CopyImage() error = %s path error for %s: %v", f, checkDir, err)
		}
	}
	return nil
}

func testCheckCopyFile(filename string) error {
	var manifestExists, versionExists bool
	err := util.NewTarGzReader(filename, func(h *tar.Header, r *tar.Reader) (bool, error) {
		switch h.Name {
		case "/version":
			versionExists = true
		case "/manifest.json":
			manifestExists = true
		}
		return false, nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	if !manifestExists {
		return fmt.Errorf("no manifest file in %s", filename)
	}

	if !versionExists {
		return fmt.Errorf("no version file in %s", filename)
	}

	return nil
}
