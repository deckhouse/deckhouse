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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"k8s.io/klog/v2"
)

func newLoadHandler(baseDir string) *loadHandler {
	return &loadHandler{baseDir}
}

type loadHandler struct {
	baseDir string
}

func (u *loadHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	pathVars := mux.Vars(request)
	channels := strings.Split(request.URL.Query().Get("channels"), ",")

	err := u.upload(request.Body, pathVars["moduleName"], pathVars["version"])
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = u.generateChannelMapping(pathVars["version"], channels)
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
}

func (u *loadHandler) upload(body io.ReadCloser, moduleName, version string) error {
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

		path := filepath.Join(u.baseDir, moduleName, version, header.Name)
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

func (u *loadHandler) generateChannelMapping(version string, channels []string) error {
	if len(channels) == 0 {
		return nil
	}

	path := filepath.Join(u.baseDir, "channels.json")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}

	var m = make(map[string]string)

	err = json.NewDecoder(f).Decode(&m)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode json: %w", err)
	}

	for _, ch := range channels {
		m[ch] = version
	}

	err = f.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncate %q: %w", path, err)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek %q: %w", path, err)
	}

	err = json.NewEncoder(f).Encode(m)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}
