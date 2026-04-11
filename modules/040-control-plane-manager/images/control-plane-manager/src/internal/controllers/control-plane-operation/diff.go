/*
Copyright 2026 Flant JSC

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

package controlplaneoperation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type diffEntry struct {
	name  string
	mtime time.Time
}

func saveDiffResults(component controlplanev1alpha1.OperationComponent, operationName string, results []fileWriteResult, logger *log.Logger) {
	for i := range results {
		res := results[i]
		if !res.Changed || res.Diff == "" {
			continue
		}
		if err := saveDiff(component, operationName, res); err != nil {
			logger.Warn("failed to save diff", log.Err(err), "file", res.Path)
		}
	}
}

func saveDiff(component controlplanev1alpha1.OperationComponent, operationName string, result fileWriteResult) error {
	opDir, err := ensureOperationDiffDir(component, operationName)
	if err != nil {
		return err
	}

	dst := filepath.Join(opDir, diffSubdirForPath(result.Path), filepath.Base(result.Path)+".diff")
	if err := writeFileAtomically(dst, []byte(result.Diff), 0o600); err != nil {
		return fmt.Errorf("write diff %s: %w", dst, err)
	}
	return nil
}

func ensureOperationDiffDir(component controlplanev1alpha1.OperationComponent, operationName string) (string, error) {
	componentDir := filepath.Join(constants.DiffBasePath, string(component))
	if err := os.MkdirAll(componentDir, 0o700); err != nil {
		return "", fmt.Errorf("create component diff dir: %w", err)
	}

	existing, err := findExistingOperationDiffDir(componentDir, operationName)
	if err != nil {
		return "", err
	}
	if existing != "" {
		return existing, nil
	}

	dirName := fmt.Sprintf("%s__%s", time.Now().UTC().Format("2006-01-02T15-04-05"), operationName)
	opDir := filepath.Join(componentDir, dirName)
	if err := os.MkdirAll(opDir, 0o700); err != nil {
		return "", fmt.Errorf("create operation diff dir: %w", err)
	}

	if err := rotateDiffs(componentDir, constants.MaxDiffsPerComponent); err != nil {
		return "", fmt.Errorf("rotate diffs: %w", err)
	}

	return opDir, nil
}

func findExistingOperationDiffDir(componentDir, operationName string) (string, error) {
	entries, err := os.ReadDir(componentDir)
	if err != nil {
		return "", fmt.Errorf("read component diff dir: %w", err)
	}

	var matches []diffEntry
	suffix := "__" + operationName
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return "", fmt.Errorf("stat diff dir %s: %w", e.Name(), err)
		}
		matches = append(matches, diffEntry{name: e.Name(), mtime: info.ModTime()})
	}
	if len(matches) == 0 {
		return "", nil
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].mtime.After(matches[j].mtime)
	})
	return filepath.Join(componentDir, matches[0].name), nil
}

func rotateDiffs(componentDiffDir string, keep int) error {
	entries, err := os.ReadDir(componentDiffDir)
	if err != nil {
		return fmt.Errorf("read diff dir: %w", err)
	}

	var dirs []diffEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("stat diff dir %s: %w", e.Name(), err)
		}
		dirs = append(dirs, diffEntry{name: e.Name(), mtime: info.ModTime()})
	}
	if len(dirs) <= keep {
		return nil
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].mtime.After(dirs[j].mtime)
	})

	for _, d := range dirs[keep:] {
		if err := os.RemoveAll(filepath.Join(componentDiffDir, d.name)); err != nil {
			return fmt.Errorf("remove old diff %s: %w", d.name, err)
		}
	}
	return nil
}

func diffSubdirForPath(path string) string {
	manifestPrefix := constants.ManifestsPath + string(os.PathSeparator)
	extraPrefix := constants.ExtraFilesPath + string(os.PathSeparator)
	switch {
	case strings.HasPrefix(path, manifestPrefix):
		return "manifests"
	case strings.HasPrefix(path, extraPrefix):
		return "extra-files"
	default:
		return "files"
	}
}
