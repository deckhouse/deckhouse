// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package script

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/shell-operator/pkg/executor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// scriptResult is the outcome of one enabled-script run: whether the module
// should be enabled and, when it should not, the human-readable reason the
// script wrote to MODULE_ENABLED_REASON.
type scriptResult struct {
	enabled bool
	reason  string
}

// runScript executes a module's enabled script and reports its verdict. It
// follows the addon-operator enabled-script protocol: settings and values are
// written to temporary JSON files exposed to the script as CONFIG_VALUES_PATH
// and VALUES_PATH, and the script writes its boolean verdict to
// MODULE_ENABLED_RESULT and an optional reason to MODULE_ENABLED_REASON. All
// four temporary files are removed before returning. It errors if a temp file
// cannot be prepared, the script exits non-zero, or the result cannot be parsed.
func runScript(ctx context.Context, path string, settings, values addonutils.Values, logger *log.Logger) (*scriptResult, error) {
	_, span := otel.Tracer("enabled-script").Start(ctx, "RunEnabledScript")
	defer span.End()

	span.SetAttributes(attribute.String("path", path))

	logger.Info("run enabled script", slog.String("path", path))

	settingsFile, err := prepareTmpFile(os.TempDir(), settings)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tmp settings file: %w", err)
	}

	defer func() {
		if err = os.Remove(settingsFile); err != nil {
			logger.Error("remove tmp settings file", slog.String("path", settingsFile), log.Err(err))
		}
	}()

	valuesFile, err := prepareTmpFile(os.TempDir(), values)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tmp values file: %w", err)
	}

	defer func() {
		if err = os.Remove(valuesFile); err != nil {
			logger.Error("remove tmp values file", slog.String("path", valuesFile), log.Err(err))
		}
	}()

	resultPath, err := prepareTmpEmptyFile(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tmp result file: %w", err)
	}

	defer func() {
		if err = os.Remove(resultPath); err != nil {
			logger.Error("remove tmp result file", slog.String("path", resultPath), log.Err(err))
		}
	}()

	reasonPath, err := prepareTmpEmptyFile(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tmp reason file: %w", err)
	}

	defer func() {
		if err = os.Remove(reasonPath); err != nil {
			logger.Error("remove tmp reason file", slog.String("path", reasonPath), log.Err(err))
		}
	}()

	environ := os.Environ()

	envs := make([]string, 0, len(environ))
	envs = append(envs, environ...)

	envs = append(envs, fmt.Sprintf("CONFIG_VALUES_PATH=%s", settingsFile))
	envs = append(envs, fmt.Sprintf("VALUES_PATH=%s", valuesFile))
	envs = append(envs, fmt.Sprintf("MODULE_ENABLED_RESULT=%s", resultPath))
	envs = append(envs, fmt.Sprintf("MODULE_ENABLED_REASON=%s", reasonPath))

	cmd := executor.NewExecutor(
		"",
		path,
		[]string{},
		envs).
		WithLogger(logger.Named("executor")).
		WithCMDStdout(nil)

	if _, err = cmd.RunAndLogLines(ctx, make(map[string]string)); err != nil {
		logger.Error("failed to run enabled script", slog.String("path", path), log.Err(err))

		return nil, fmt.Errorf("failed to run enabled script: %w", err)
	}

	result, err := parseScriptResult(resultPath, reasonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse script result: %w", err)
	}

	return &result, nil
}

// prepareTmpFile writes the given values as JSON into a fresh, uniquely named
// file under dir and returns its path. The file is created with owner-only
// permissions (0o600) because it may carry secret module values.
func prepareTmpFile(dir string, settings addonutils.Values) (string, error) {
	data, err := settings.JsonBytes()
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp(dir, "*.json")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}

	return file.Name(), nil
}

// prepareTmpEmptyFile creates a fresh, uniquely named empty file under dir for
// the script to write its result into and returns its path. Like prepareTmpFile
// it uses owner-only permissions (0o600).
func prepareTmpEmptyFile(dir string) (string, error) {
	file, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	return file.Name(), nil
}

// parseScriptResult reads the verdict and reason the script left in its two
// output files and combines them into a scriptResult.
func parseScriptResult(resultPath, reasonPath string) (scriptResult, error) {
	enabled, err := readScriptResult(resultPath)
	if err != nil {
		return scriptResult{}, err
	}

	reason, err := readScriptReason(reasonPath)
	if err != nil {
		return scriptResult{}, err
	}

	return scriptResult{enabled: enabled, reason: reason}, nil
}

// readScriptResult reads MODULE_ENABLED_RESULT and parses its single trimmed
// token, which must be exactly "true" or "false"; any other content is an error.
func readScriptResult(filePath string) (bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("read %s: %s", filePath, err)
	}

	value := strings.TrimSpace(string(data))

	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	return false, fmt.Errorf("expected 'true' or 'false', got '%s'", value)
}

// readScriptReason reads MODULE_ENABLED_REASON and returns its trimmed contents.
// The reason is optional, so an empty file yields an empty string.
func readScriptReason(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %s", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}
