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
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/models/hooks"
	"github.com/flant/addon-operator/pkg/module_manager/models/hooks/kind"
	"github.com/flant/addon-operator/pkg/utils"
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

// hookLoader handles discovery and loading of package hooks from the filesystem.
// It supports both shell hooks (.sh, .py) and batch hooks (executables).
// This causes executable hooks to be rejected and non-executable files to be accepted.
type hookLoader struct {
	packageName string // Package name for hook context
	path        string // Package directory path
	keepTmp     bool   // Whether to keep temporary files for debugging

	// readinessLoaded tracks if a readiness hook was found
	readinessLoaded bool

	logger *log.Logger
}

// newHookLoader creates a new hook loader for the specified package.
func newHookLoader(packageName, path string, keepTmp bool, logger *log.Logger) *hookLoader {
	return &hookLoader{
		packageName: packageName,
		path:        path,
		keepTmp:     keepTmp,

		logger: logger,
	}
}

// load discovers and loads all package hooks from the filesystem.
// It searches for both shell hooks (.sh, .py) and batch hooks (executables).
func (l *hookLoader) load(ctx context.Context) ([]*hooks.ModuleHook, error) {
	_, span := otel.Tracer(hooksLoaderTracer).Start(ctx, "load")
	defer span.End()

	span.SetAttributes(attribute.String("package", l.packageName))
	span.SetAttributes(attribute.String("path", l.path))

	l.logger.Debug("load hooks")

	packagesHooks, err := l.searchPackageHooks()
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("search hooks failed: %w", err)
	}

	l.logger.Debug("found hooks", slog.Int("count", len(packagesHooks)))

	return packagesHooks, nil
}

func (l *hookLoader) searchPackageHooks() ([]*hooks.ModuleHook, error) {
	shellHooks, err := l.searchPackageShellHooks()
	if err != nil {
		return nil, fmt.Errorf("search shell hooks: %w", err)
	}

	batchHooks, err := l.searchPackageBatchHooks()
	if err != nil {
		return nil, fmt.Errorf("search batch hooks: %w", err)
	}

	result := make([]*hooks.ModuleHook, 0, len(shellHooks)+len(batchHooks))

	for _, h := range shellHooks {
		result = append(result, hooks.NewModuleHook(h))
	}

	for _, h := range batchHooks {
		result = append(result, hooks.NewModuleHook(h))
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].GetPath() < result[j].GetPath()
	})

	return result, nil
}

func (l *hookLoader) searchPackageShellHooks() ([]*kind.ShellHook, error) {
	hooksPath := filepath.Join(l.path, hooksDir)
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		return nil, nil
	}

	hooksRelativePaths, err := l.getHookExecutablePaths(hooksDir, false)
	if err != nil {
		return nil, err
	}

	result := make([]*kind.ShellHook, 0)

	// sort hooks by path
	sort.Strings(hooksRelativePaths)

	var (
		checkPythonEnv           sync.Once
		discoveredPythonVenvPath string
	)

	for _, hookPath := range hooksRelativePaths {
		options := make([]kind.ShellHookOption, 0, 1)

		if filepath.Ext(hookPath) == ".py" {
			checkPythonEnv.Do(func() {
				f, err := os.Stat(filepath.Join(l.path, kind.PythonVenvPath, kind.PythonBinaryPath))
				if err == nil {
					if !f.IsDir() && f.Mode()&0o111 != 0 {
						discoveredPythonVenvPath = filepath.Join(l.path, kind.PythonVenvPath)
					}
				}
			})
			options = append(options, kind.WithPythonVenv(discoveredPythonVenvPath))
		}

		hookName, err := normalizeHookPath(filepath.Dir(l.path), hookPath)
		if err != nil {
			return nil, fmt.Errorf("get hook name: %w", err)
		}

		if filepath.Ext(hookPath) == "" {
			if _, err = kind.GetBatchHookConfig(l.packageName, hookPath); err == nil {
				continue
			}

			l.logger.Warn("get batch hook config", slog.String("hook_file_path", hookPath), log.Err(err))
		}

		logger := l.logger.Named("shell-hook")
		hook := kind.NewShellHook(hookName,
			hookPath, l.packageName, l.keepTmp,
			shapp.LogProxyHookJSON, logger, options...)

		result = append(result, hook)
	}

	return result, nil
}

func (l *hookLoader) searchPackageBatchHooks() ([]*kind.BatchHook, error) {
	hooksPath := filepath.Join(l.path, hooksDir)
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		return nil, nil
	}

	hooksRelativePaths, err := l.getHookExecutablePaths(hooksDir, true)
	if err != nil {
		return nil, err
	}

	result := make([]*kind.BatchHook, 0)

	// sort hooks by path
	sort.Strings(hooksRelativePaths)

	for _, hookPath := range hooksRelativePaths {
		hookName, err := normalizeHookPath(filepath.Dir(l.path), hookPath)
		if err != nil {
			return nil, fmt.Errorf("get hook name: %w", err)
		}

		hookConfig, err := kind.GetBatchHookConfig(l.packageName, hookPath)
		if err != nil {
			return nil, fmt.Errorf("get sdk config for hook '%s': %w", hookName, err)
		}

		if hookConfig.Readiness != nil {
			if l.readinessLoaded {
				return nil, fmt.Errorf("multiple readiness hooks found in '%s'", hookPath)
			}

			l.readinessLoaded = true

			// add readiness hook
			nestedHookName := fmt.Sprintf("%s-readiness", hookName)
			logger := l.logger.Named("batch-hook")

			hook := kind.NewBatchHook(nestedHookName,
				hookPath, l.packageName, kind.BatchHookReadyKey,
				l.keepTmp, shapp.LogProxyHookJSON, logger)

			result = append(result, hook)
		}

		for key, cfg := range hookConfig.Hooks {
			nestedHookName := fmt.Sprintf("%s:%s:%s", hookName, cfg.Metadata.Name, key)
			logger := l.logger.Named("batch-hook")

			hook := kind.NewBatchHook(nestedHookName,
				hookPath, l.packageName, key,
				l.keepTmp, shapp.LogProxyHookJSON, logger)

			result = append(result, hook)
		}
	}

	return result, nil
}

func (l *hookLoader) getHookExecutablePaths(dir string, checkBatch bool) ([]string, error) {
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
			log.Debug("file is skipped", slog.String("path", path), log.Err(err))
			return nil
		}

		if checkBatch {
			if err = isExecutableBatchHook(l.packageName, path, f); err != nil {
				l.logger.Debug("skip file", slog.String("path", path), log.Err(err))

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

func isExecutableBatchHook(name, path string, file os.FileInfo) error {
	if err := isExecutable(file); err != nil {
		return err
	}

	switch filepath.Ext(file.Name()) {
	// ignore any extension and hidden files
	case "":
		return isBatchHook(name, path)
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
func isBatchHook(moduleName, path string) error {
	// TODO: check binary another way
	args := []string{"hook", "list"}

	// Execute the binary to check if it's a batch hook
	cmd := executor.NewExecutor(
		"",
		path,
		args,
		[]string{}).
		WithChroot(utils.GetModuleChrootPath(moduleName))

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
