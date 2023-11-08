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
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func UnpackBundle(mirrorCtx *Context) error {
	tarFile, err := os.Open(mirrorCtx.TarBundlePath)
	if err != nil {
		return fmt.Errorf("read tar bundle: %w", err)
	}

	tarReader := tar.NewReader(bufio.NewReaderSize(tarFile, 128*1024)) // 128 KiB
	for {
		tarHdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		writePath := filepath.Join(mirrorCtx.UnpackedImagesPath, filepath.Clean(tarHdr.Name))
		if err = os.MkdirAll(filepath.Dir(writePath), 0755); err != nil {
			return fmt.Errorf("setup dir tree: %w", err)
		}
		bundleFile, err := os.OpenFile(writePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		if _, err = io.Copy(bundleFile, tarReader); err != nil {
			return fmt.Errorf("write %q: %w", writePath, err)
		}
		if err = bundleFile.Sync(); err != nil {
			return fmt.Errorf("write %q: %w", writePath, err)
		}
		if err = bundleFile.Close(); err != nil {
			return fmt.Errorf("write %q: %w", writePath, err)
		}
	}

	return nil
}

func PackBundle(mirrorCtx *Context) error {
	tarFile, err := os.Create(mirrorCtx.TarBundlePath)
	if err != nil {
		return fmt.Errorf("read tar bundle: %w", err)
	}
	defer tarFile.Close()

	tarWriter := tar.NewWriter(tarFile)
	if err = filepath.Walk(mirrorCtx.UnpackedImagesPath, packFunc(mirrorCtx, tarWriter)); err != nil {
		return fmt.Errorf("pack mirrored images into tar: %w", err)
	}

	if err = tarFile.Sync(); err != nil {
		return fmt.Errorf("write tar archive: %w", err)
	}

	return nil

}

func packFunc(mirrorCtx *Context, out *tar.Writer) filepath.WalkFunc {
	return func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == mirrorCtx.TarBundlePath || info.IsDir() {
			return nil
		}

		blobFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		pathInTar, err := filepath.Rel(mirrorCtx.UnpackedImagesPath, path)
		if err != nil {
			return fmt.Errorf("build file path within bundle: %w", err)
		}
		err = out.WriteHeader(&tar.Header{
			Name:    pathInTar,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		})
		if err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}

		if _, err = io.Copy(out, bufio.NewReaderSize(blobFile, 512*1024)); err != nil {
			return fmt.Errorf("write file to tar: %w", err)
		}

		if err = blobFile.Close(); err != nil {
			return fmt.Errorf("close file descriptor: %w", err)
		}

		// We don't care about error here.
		// Whole folder with unpacked images will be deleted after bundle is packed.
		//
		// We attempt to delete packed parts of layout here only to save some storage space,
		// avoiding duplication of data that was already written to tar bundle.
		_ = os.Remove(path)

		return nil
	}
}
