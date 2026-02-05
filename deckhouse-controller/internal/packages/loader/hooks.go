// Copyright 2025 Flant JSC
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

package loader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	shapp "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/executor"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// hooksDir is the subdirectory name containing hook scripts
	hooksDir = "hooks"

	hooksLoaderTracer = "hooks-loader"
)

var (
	// ErrFileWrongExtension is returned when a file has an unexpected extension for a batch hook
	ErrFileWrongExtension = errors.New("file has wrong extension")
	// ErrFileNotBatchHook is returned when a file doesn't respond correctly to "hook list"
	ErrFileNotBatchHook = errors.New("file is not batch hook")
	// ErrFileNotExecutable is returned when a hook file lacks executable permissions
	ErrFileNotExecutable = errors.New("no executable permissions, chmod +x is required to run this hook")

	// the list of subdirectories to exclude when searching for a module's hooks
	hooksExcludedDir = []string{"venv", "lib"}

	// compiledHooksFound matches the output of batch hooks' "hook list" command
	compiledHooksFound = regexp.MustCompile(`Found ([1-9]|[1-9]\d|[1-9]\d\d|[1-9]\d\d\d) items`)
)

type hookLoadResult struct {
	settingsCheck *kind.SettingsCheck
	hooks         []*hooks.ModuleHook
}

// loadAppHooks discovers and loads all package hooks from the filesystem.
// It searches for both shell hooks (.sh, .py) and batch hooks (executables).
func loadAppHooks(ctx context.Context, namespace, name, path string, logger *log.Logger) (*hookLoadResult, error) {
	_, span := otel.Tracer(hooksLoaderTracer).Start(ctx, "load")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("namespace", namespace))
	span.SetAttributes(attribute.String("path", path))

	logger.Debug("load hooks")

	res, err := searchBatchAppHooks(namespace, name, path, logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return res, fmt.Errorf("search hooks failed: %w", err)
	}

	logger.Debug("found hooks", slog.Int("count", len(res.hooks)))

	return res, nil
}

// loadModuleHooks discovers and loads all package hooks from the filesystem.
// It searches for both shell hooks (.sh, .py) and batch hooks (executables).
func loadModuleHooks(ctx context.Context, name, path string, logger *log.Logger) (*hookLoadResult, error) {
	_, span := otel.Tracer(hooksLoaderTracer).Start(ctx, "load")
	defer span.End()

	span.SetAttributes(attribute.String("name", name))
	span.SetAttributes(attribute.String("path", path))

	logger.Debug("load hooks")

	res, err := searchBatchModuleHooks(name, path, logger)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return res, fmt.Errorf("search hooks failed: %w", err)
	}

	logger.Debug("found hooks", slog.Int("count", len(res.hooks)))

	return res, nil
}

func searchBatchAppHooks(namespace, name, path string, logger *log.Logger) (*hookLoadResult, error) {
	hooksPath := filepath.Join(path, hooksDir)
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		return &hookLoadResult{}, nil
	}

	hooksRelativePaths, err := getHookExecutablePaths(hooksPath, true, logger)
	if err != nil {
		return nil, err
	}

	result := new(hookLoadResult)

	readinessLoaded := false

	// sort hooks by path
	sort.Strings(hooksRelativePaths)

	for _, hookPath := range hooksRelativePaths {
		hookName, err := normalizeHookPath(filepath.Dir(path), hookPath)
		if err != nil {
			return nil, fmt.Errorf("get hook name: %w", err)
		}

		hookConfig, err := kind.GetBatchHookConfig(name, hookPath)
		if err != nil {
			return nil, fmt.Errorf("get sdk config for hook '%s': %w", hookName, err)
		}

		if hookConfig.Readiness != nil {
			if readinessLoaded {
				return nil, fmt.Errorf("multiple readiness hooks found in '%s'", hookPath)
			}

			readinessLoaded = true

			// add readiness hook
			nestedHookName := fmt.Sprintf("%s-readiness", hookName)
			hookLogger := logger.Named("batch-hook")

			hook := kind.NewApplicationBatchHook(nestedHookName,
				hookPath, namespace, name, kind.BatchHookReadyKey,
				shapp.DebugKeepTmpFiles, shapp.LogProxyHookJSON, hookLogger)

			result.hooks = append(result.hooks, hooks.NewModuleHook(hook))
		}

		if hookConfig.HasSettingsCheck {
			if result.settingsCheck != nil {
				return nil, fmt.Errorf("multiple settings checks found in '%s'", hookPath)
			}

			settingsLogger := logger.Named("settings-check")
			result.settingsCheck = kind.NewSettingsCheck(hookPath, os.TempDir(), settingsLogger)
		}

		for key, cfg := range hookConfig.Hooks {
			nestedHookName := fmt.Sprintf("%s:%s:%s", hookName, cfg.Metadata.Name, key)
			hookLogger := logger.Named("batch-hook")

			hook := kind.NewApplicationBatchHook(nestedHookName,
				hookPath, namespace, name, key,
				shapp.DebugKeepTmpFiles, shapp.LogProxyHookJSON, hookLogger)

			result.hooks = append(result.hooks, hooks.NewModuleHook(hook))
		}
	}

	return result, nil
}

func searchBatchModuleHooks(name, path string, logger *log.Logger) (*hookLoadResult, error) {
	hooksPath := filepath.Join(path, hooksDir)
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		return &hookLoadResult{}, nil
	}

	hooksRelativePaths, err := getHookExecutablePaths(hooksPath, true, logger)
	if err != nil {
		return nil, err
	}

	result := new(hookLoadResult)

	readinessLoaded := false

	// sort hooks by path
	sort.Strings(hooksRelativePaths)

	for _, hookPath := range hooksRelativePaths {
		hookName, err := normalizeHookPath(filepath.Dir(path), hookPath)
		if err != nil {
			return nil, fmt.Errorf("get hook name: %w", err)
		}

		hookConfig, err := kind.GetBatchHookConfig(name, hookPath)
		if err != nil {
			return nil, fmt.Errorf("get sdk config for hook '%s': %w", hookName, err)
		}

		if hookConfig.Readiness != nil {
			if readinessLoaded {
				return nil, fmt.Errorf("multiple readiness hooks found in '%s'", hookPath)
			}

			readinessLoaded = true

			// add readiness hook
			nestedHookName := fmt.Sprintf("%s-readiness", hookName)
			hookLogger := logger.Named("batch-hook")

			hook := kind.NewBatchHook(nestedHookName,
				hookPath, name, kind.BatchHookReadyKey,
				shapp.DebugKeepTmpFiles, shapp.LogProxyHookJSON, hookLogger)

			result.hooks = append(result.hooks, hooks.NewModuleHook(hook))
		}

		if hookConfig.HasSettingsCheck {
			if result.settingsCheck != nil {
				return nil, fmt.Errorf("multiple settings checks found in '%s'", hookPath)
			}

			settingsLogger := logger.Named("settings-check")
			result.settingsCheck = kind.NewSettingsCheck(hookPath, os.TempDir(), settingsLogger)
		}

		for key, cfg := range hookConfig.Hooks {
			nestedHookName := fmt.Sprintf("%s:%s:%s", hookName, cfg.Metadata.Name, key)
			hookLogger := logger.Named("batch-hook")

			hook := kind.NewBatchHook(nestedHookName,
				hookPath, name, key,
				shapp.DebugKeepTmpFiles, shapp.LogProxyHookJSON, hookLogger)

			result.hooks = append(result.hooks, hooks.NewModuleHook(hook))
		}
	}

	return result, nil
}

func getHookExecutablePaths(dir string, checkBatch bool, logger *log.Logger) ([]string, error) {
	paths := make([]string, 0)

	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if f.IsDir() {
			// Skip hidden and lib directories inside initial directory
			if strings.HasPrefix(f.Name(), ".") || slices.Contains(hooksExcludedDir, f.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if err = isExecutable(f); err != nil {
			logger.Debug("file is skipped", slog.String("path", path), log.Err(err))
			return nil
		}

		if checkBatch {
			if err = isExecutableBatchHook(path, f); err != nil {
				logger.Debug("skip file", slog.String("path", path), log.Err(err))

				return nil
			}
		}

		paths = append(paths, path)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

func isExecutableBatchHook(path string, file os.FileInfo) error {
	if err := isExecutable(file); err != nil {
		return err
	}

	switch filepath.Ext(file.Name()) {
	// ignore any extension and hidden files
	case "":
		return isBatchHook(path)
	// ignore all with extensions
	default:
		return ErrFileWrongExtension
	}
}

// isBatchHook determines if a binary is a batch hook by executing it.
// It runs the binary with "hook list" and checks if the output matches the expected format.
//
// WARNING: Security issue - executes untrusted binaries during discovery
// WARNING: Performance issue - runs every executable file found
// TODO: Consider alternative detection methods (file signatures, metadata, etc.)
func isBatchHook(path string) error {
	// TODO: check binary another way
	args := []string{"hook", "list"}

	// Execute the binary to check if it's a batch hook
	cmd := executor.NewExecutor(
		"",
		path,
		args,
		[]string{})

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("exec file '%s': %w", path, err)
	}

	// Check if output matches expected batch hook format
	if compiledHooksFound.Match(out) {
		return nil
	}

	return ErrFileNotBatchHook
}

// isExecutable checks if a file has executable permissions.
func isExecutable(file os.FileInfo) error {
	if file.Mode()&0o111 != 0 {
		return nil
	}

	return ErrFileNotExecutable
}

func normalizeHookPath(modulePath, hookPath string) (string, error) {
	hooksIdx := strings.Index(hookPath, "/hooks/")
	if hooksIdx == -1 {
		return filepath.Rel(modulePath, hookPath)
	}
	relPath := hookPath[hooksIdx+1:]

	return relPath, nil
}
