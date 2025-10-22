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

package docs

import (
	"archive/tar"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flant/docs-builder/internal/metrics"
)

func (svc *Service) Upload(body io.ReadCloser, moduleName string, version string, channels []string) error {
	start := time.Now()
	status := "ok"
	defer func() {
		dur := time.Since(start).Seconds()
		svc.metrics.CounterAdd(metrics.DocsBuilderUploadTotal, 1, map[string]string{"status": status})
		svc.metrics.HistogramObserve(metrics.DocsBuilderUploadDurationSeconds, dur, map[string]string{"status": status}, nil)
	}()

	err := svc.cleanModulesFiles(moduleName, channels)
	if err != nil {
		status = "fail"

		return fmt.Errorf("clean module files: %w", err)
	}

	reader := tar.NewReader(body)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			svc.logger.Info("EOF reading file")
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
				path, ok := svc.getLocalPath(moduleName, channel, header.Name)
				if !ok {
					svc.logger.Info("skipping tree", slog.String("headerName", header.Name), slog.String("moduleName", moduleName))
					continue
				}

				svc.logger.Info("creating dir", slog.String("path", path))
				if err := os.MkdirAll(path, 0700); err != nil {
					return fmt.Errorf("mkdir %q failed: %w", path, err)
				}
			}
		case tar.TypeReg:
			files := make([]io.Writer, 0, len(channels))

			for _, channel := range channels {
				path, ok := svc.getLocalPath(moduleName, channel, header.Name)
				if !ok {
					svc.logger.Info("skipping file", slog.String("header_name", header.Name), slog.String("module_name", moduleName))
					continue
				}
				svc.logger.Info("creating file", slog.String("path", path))

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
			return fmt.Errorf("extract unknown type: %d in %s", header.Typeflag, header.Name)
		}
	}

	err = svc.generateChannelMapping(moduleName, version, channels)
	if err != nil {
		return fmt.Errorf("generate error mapping: %w", err)
	}

	return nil
}

func (svc *Service) generateChannelMapping(moduleName, version string, channels []string) error {
	return svc.channelMappingEditor.edit(func(m channelMapping) {
		var versions = make(map[string]versionEntity)
		if _, ok := m[moduleName]; ok {
			versions = m[moduleName][channelMappingChannels]
		}

		for _, ch := range channels {
			versions[ch] = versionEntity{version}
		}

		m[moduleName] = map[string]map[string]versionEntity{
			channelMappingChannels: versions,
		}
	})
}

func (svc *Service) getLocalPath(moduleName, channel, fileName string) (string, bool) {
	fileName = filepath.Clean(fileName)

	if strings.HasSuffix(fileName, "_RU.md") {
		fileName = strings.Replace(fileName, "_RU.md", ".ru.md", 1)
	}

	if fileName, ok := strings.CutPrefix(fileName, "docs"); ok {
		return filepath.Join(svc.baseDir, contentDir, moduleName, channel, fileName), true
	}

	if strings.HasPrefix(fileName, "crds") ||
		fileName == "openapi" ||
		fileName == "openapi/conversions" ||
		fileName == "openapi/config-values.yaml" ||
		docConfValuesRegexp.MatchString(fileName) {
		return filepath.Join(svc.baseDir, modulesDir, moduleName, channel, fileName), true
	}

	return "", false
}

func (svc *Service) cleanModulesFiles(moduleName string, channels []string) error {
	for _, channel := range channels {
		path := filepath.Join(svc.baseDir, contentDir, moduleName, channel)
		err := os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("remove content %s: %w", path, err)
		}

		path = filepath.Join(svc.baseDir, modulesDir, moduleName, channel)
		err = os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("remove data %s: %w", path, err)
		}
	}

	return nil
}
