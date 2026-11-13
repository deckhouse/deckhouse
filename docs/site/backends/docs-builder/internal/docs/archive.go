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
)

// unpack places the files of a module documentation archive into the Hugo
// site as the given version of the module. The archive is bounded by
// MaxDocsBytes and MaxDocsFiles: it is built from an image of a registry.
func (svc *Service) unpack(body io.Reader, moduleName, version string) error {
	reader := tar.NewReader(body)

	var size DocsSize
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
			path, ok := svc.getLocalPath(moduleName, version, header.Name)
			if !ok {
				svc.logger.Info("skipping tree", slog.String("header_name", header.Name), slog.String("module_name", moduleName))
				continue
			}

			svc.logger.Info("creating dir", slog.String("path", path))
			if err := os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("mkdir %q failed: %w", path, err)
			}
		case tar.TypeReg:
			if err := size.Add(header.Size); err != nil {
				return err
			}

			path, ok := svc.getLocalPath(moduleName, version, header.Name)
			if !ok {
				svc.logger.Info("skipping file", slog.String("header_name", header.Name), slog.String("module_name", moduleName))
				continue
			}
			svc.logger.Info("creating file", slog.String("path", path))

			// keep only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err := writeFile(path, os.FileMode(header.Mode)&0700, reader); err != nil {
				return err
			}

		default:
			return fmt.Errorf("extract unknown type: %d in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

// writeFile writes body to path. A tar archive is not guaranteed to carry a
// directory entry before each file (e.g. root-level module.yaml/oss.yaml
// have none), so the parent directory is created first.
func writeFile(path string, mode os.FileMode, body io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("mkdir %q failed: %w", filepath.Dir(path), err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %q failed: %w", path, err)
	}

	_, err = io.Copy(f, body)
	// A failed close may lose the data written.
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("write %q failed: %w", path, err)
	}

	return nil
}

func (svc *Service) getLocalPath(moduleName, version, fileName string) (string, bool) {
	fileName = filepath.Clean(fileName)

	if strings.HasSuffix(fileName, "_RU.md") {
		fileName = strings.Replace(fileName, "_RU.md", ".ru.md", 1)
	}

	checked := func(path string) (string, bool) {
		if err := ensureWithinBase(svc.baseDir, path); err != nil {
			svc.logger.Warn("skipping path outside base directory", slog.String("path", path), slog.String("error", err.Error()))

			return "", false
		}

		return path, true
	}

	if fileName, ok := strings.CutPrefix(fileName, "docs"); ok {
		// Skip internal documentation directories that should not be published
		if hasBlockedPrefix(fileName) {
			return "", false
		}

		return checked(filepath.Join(svc.baseDir, contentDir, moduleName, version, fileName))
	}

	if fileName == "candi/openapi" ||
		fileName == "candi/openapi/cluster_configuration.yaml" ||
		fileName == "candi/openapi/doc-ru-cluster_configuration.yaml" {
		return checked(filepath.Join(svc.baseDir, modulesDir, moduleName, version, fileName))
	}

	if isAllowedCRDPath(fileName) ||
		fileName == "openapi" ||
		fileName == "openapi/conversions" ||
		fileName == "openapi/config-values.yaml" ||
		docConfValuesRegexp.MatchString(fileName) {
		return checked(filepath.Join(svc.baseDir, modulesDir, moduleName, version, fileName))
	}

	if fileName == "module.yaml" || fileName == "oss.yaml" {
		return checked(filepath.Join(svc.baseDir, modulesDir, moduleName, version, fileName))
	}

	return "", false
}

// isAllowedCRDPath reports whether the given path under crds/ may be uploaded
// to Hugo's data directory.
//
// Two constraints shape the rule:
//
//  1. Hugo loads everything in data/ as structured data and supports only
//     yaml/yml/json/toml/xml formats. Non-data files (e.g. crds/README.md,
//     crds/update.sh) fail the whole build with
//     `unmarshal of format "" is not supported` and the module gets dropped
//     as "broken".
//  2. The docs-builder-template renders crds/ as a flat map — one file per
//     CRD spec (see layouts/_partials/module-resources.html and
//     openapi/format-crd.html). Subdirectories like crds/native/foo.yaml
//     would be loaded into a nested map and silently corrupt the CRD section
//     of the page.
//
// So we accept only:
//   - the crds directory entry itself (needed so MkdirAll can create it);
//   - direct children of crds/ with a data extension (.yaml/.yml/.json).
//
// Everything else (subdirectories and non-data files at any depth) is
// rejected.
func isAllowedCRDPath(path string) bool {
	if path == "crds" {
		return true
	}

	rest, ok := strings.CutPrefix(path, "crds/")
	if !ok || strings.Contains(rest, "/") {
		return false
	}

	switch filepath.Ext(rest) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func hasBlockedPrefix(path string) bool {
	blockedDocPathPrefixes := []string{
		"/internal",
		"/internals",
		"/development",
		"/dev",
	}
	for _, prefix := range blockedDocPathPrefixes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}

		if len(path) == len(prefix) {
			return true
		}

		if path[len(prefix)] == '/' {
			return true
		}
	}

	return false
}
