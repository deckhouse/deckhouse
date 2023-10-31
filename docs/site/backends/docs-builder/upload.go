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
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
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
	if len(channels) == 0 {
		channels = []string{"stable"}
	}

	err := u.upload(request.Body, pathVars["moduleName"], channels)
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = u.generateChannelMapping(pathVars["moduleName"], pathVars["version"], channels)
	if err != nil {
		klog.Error(err)
		http.Error(writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
}

func (u *loadHandler) upload(body io.ReadCloser, moduleName string, channels []string) error {
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

		switch header.Typeflag {
		case tar.TypeDir:
			for _, channel := range channels {
				path, ok := u.getLocalPath(moduleName, channel, header.Name)
				if !ok {
					klog.Infof("skipping %v in %s", header.Name, moduleName)
					continue
				}

				if err := os.MkdirAll(path, 0700); err != nil {
					return fmt.Errorf("mkdir %q failed: %w", path, err)
				}
			}
		case tar.TypeReg:
			files := make([]io.Writer, 0, len(channels))

			for _, channel := range channels {
				path, ok := u.getLocalPath(moduleName, channel, header.Name)
				if !ok {
					klog.Infof("skipping %v in %s", header.Name, moduleName)
					continue
				}
				klog.Infof("creating %s", path)

				outFile, err := os.OpenFile(
					path,
					os.O_RDWR|os.O_CREATE|os.O_TRUNC,
					os.FileMode(header.Mode)&0700, // keep only 'user' permission bit, E.x.: 644 => 600, 755 => 700
				)
				if err != nil {
					return fmt.Errorf("create %q failed: %w", path, err)
				}

				files = append(files, outFile)
			}

			if _, err := io.Copy(io.MultiWriter(files...), reader); err != nil {
				return fmt.Errorf("copy failed: %w", err)
			}

			for _, f := range files {
				f.(*os.File).Close()
			}

		default:
			return fmt.Errorf("extract uknown type: %d in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

func (u *loadHandler) generateChannelMapping(moduleName, version string, channels []string) error {
	path := filepath.Join(u.baseDir, "data/modules.yaml")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}

	type nameVersion struct {
		Name    string `json:"name"    yaml:"name"`
		Version string `json:"version" yaml:"version"`
	}

	var m = make(map[string]map[string][]nameVersion)

	err = yaml.NewDecoder(f).Decode(&m)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode json: %w", err)
	}

	versions := make([]nameVersion, 0, len(channels))
	for _, ch := range channels {
		versions = append(versions, nameVersion{ch, version})
	}

	m[moduleName] = map[string][]nameVersion{
		"channels": versions,
	}

	err = f.Truncate(0)
	if err != nil {
		return fmt.Errorf("truncate %q: %w", path, err)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek %q: %w", path, err)
	}

	err = yaml.NewEncoder(f).Encode(m)
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}

func (u *loadHandler) getLocalPath(moduleName, channel, fileName string) (string, bool) {
	fileName, _ = strings.CutPrefix(fileName, "./")

	if fileName, ok := strings.CutPrefix(fileName, "docs"); ok {
		return filepath.Join(u.baseDir, "content", moduleName, channel, fileName), true
	}

	return "", false
}
