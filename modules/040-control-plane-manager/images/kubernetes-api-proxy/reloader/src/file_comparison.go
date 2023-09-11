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

package src

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func calculateHash(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %s", err)
	}

	return hash.Sum(nil), nil
}

func fileContentsEqual(file1Path, file2Path string) (bool, error) {
	hash1, err := calculateHash(file1Path)
	if err != nil {
		return false, fmt.Errorf("error calculating hash for %s: %s\n", file1Path, err)
	}

	hash2, err := calculateHash(file2Path)
	if err != nil {
		return false, fmt.Errorf("error calculating hash for %s: %s\n", file2Path, err)
	}

	return bytes.Equal(hash1, hash2), nil
}
