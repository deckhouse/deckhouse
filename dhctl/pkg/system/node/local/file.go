// Copyright 2024 Flant JSC
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

package local

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type File struct{}

func NewFile() *File {
	return &File{}
}

func (File) Upload(_ context.Context, srcPath, dstPath string) error {
	if err := copyRecursively(srcPath, dstPath); err != nil {
		return err
	}
	return nil
}

func (File) Download(_ context.Context, srcPath, dstPath string) error {
	if err := copyRecursively(srcPath, dstPath); err != nil {
		return err
	}
	return nil
}

func (File) UploadBytes(_ context.Context, data []byte, dstPath string) error {
	if err := os.WriteFile(dstPath, data, 0666); err != nil {
		return err
	}
	return nil
}

func (File) DownloadBytes(_ context.Context, srcPath string) ([]byte, error) {
	file, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func copyRecursively(src string, dst string) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcStat.IsDir() {
		return copyFile(src, filepath.Join(dst, filepath.Base(src)))
	}

	if err = os.MkdirAll(dst, srcStat.Mode()); err != nil {
		return err
	}

	srcEntries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range srcEntries {
		srcEntryPath := filepath.Join(src, entry.Name())
		destEntryPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = copyRecursively(srcEntryPath, destEntryPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcEntryPath, destEntryPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcStat, err := srcFile.Stat()
	if err != nil {
		return err
	}

	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcStat.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, srcFile); err != nil {
		return err
	}

	if err = destFile.Sync(); err != nil {
		return err
	}

	return nil
}
