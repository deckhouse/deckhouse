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
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func saveDiffResults(component controlplanev1alpha1.OperationComponent, operationName string, results []fileWriteResult, logger *log.Logger) {
	for i := range results {
		res := results[i]
		if !res.Changed || res.Diff == "" {
			continue
		}
		if err := saveDiff(component, operationName, res); err != nil {
			logger.Warn("failed to save diff", log.Err(err), slog.String("file", res.Path))
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

	opDir := filepath.Join(componentDir, operationName)
	if err := os.MkdirAll(opDir, 0o700); err != nil {
		return "", fmt.Errorf("create operation diff dir: %w", err)
	}

	if err := rotateDiffs(componentDir, constants.MaxDiffsPerComponent); err != nil {
		return "", fmt.Errorf("rotate diffs: %w", err)
	}

	return opDir, nil
}

func rotateDiffs(componentDiffDir string, keep int) error {
	return rotateDirectories(componentDiffDir, keep)
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

func computeUnifiedDiff(oldContent, newContent, filename string) string {
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(normalizeDiffInput(oldContent)),
		B:        difflib.SplitLines(normalizeDiffInput(newContent)),
		FromFile: filename,
		ToFile:   filename + " (new)",
		Context:  3,
	})
	if err != nil {
		return ""
	}
	return diff
}

func normalizeDiffInput(content string) string {
	if content == "" {
		return ""
	}
	if strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}
