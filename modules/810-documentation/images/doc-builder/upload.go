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

package main

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

func newLoadHandler(baseDir string) *loadHandler {
	return &loadHandler{baseDir}
}

type loadHandler struct {
	baseDir string
}

// TODO: path
// TODO: generate json
func (u *loadHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	err := u.upload(request.Body)
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
}

func (u *loadHandler) upload(body io.ReadCloser) error {
	reader := tar.NewReader(body)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		if strings.Contains(header.Name, "..") {
			// CWE-22 check, prevents path traversal
			return fmt.Errorf("path traversal detected in the module archive: malicious path %v", header.Name)
		}

		path := filepath.Join(u.baseDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("mkdir %q failed: %w", path, err)
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(
				path,
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				os.FileMode(header.Mode)&0700, // remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			)
			if err != nil {
				return fmt.Errorf("create %q failed: %w", path, err)
			}
			if _, err := io.Copy(outFile, reader); err != nil {
				return fmt.Errorf("copy to %q failed: %w", path, err)
			}
			outFile.Close()

		default:
			return fmt.Errorf("extract uknown type: %v in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
