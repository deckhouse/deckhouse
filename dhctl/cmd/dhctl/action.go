// Copyright 2026 Flant JSC
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
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type onShutdownFunc func()

func doNothingOnShutdownFunc() {}

// actionIniter is set up by prepare and read by main after the command returns, on one goroutine.
type actionIniter struct {
	logFile  string
	slogRoot *slog.Logger
	opts     *options.Options
}

func newActionIniter(opts *options.Options) *actionIniter {
	return &actionIniter{opts: opts}
}

// Logger returns the application slog root, or a text-on-stderr fallback if not yet initialized.
func (i *actionIniter) Logger() *slog.Logger {
	if i.slogRoot != nil {
		return i.slogRoot
	}

	return logger.NewRoot(logger.Options{FileWriter: io.Discard, TTYWriter: os.Stderr})
}

func (i *actionIniter) init(cmd *cobra.Command) error {
	tmpDir := i.opts.Global.TmpDir
	if tmpDir == "" {
		return fmt.Errorf("Internal error: action initer not initialized. Tmp dir is empty")
	}

	stateDir := i.opts.Cache.Dir
	if stateDir == "" {
		return fmt.Errorf("Internal error: action initer not initialized. State dir is empty")
	}

	dirsToInitialize := directoriesToInitialize{
		"temp dir":  tmpDir,
		"state dir": stateDir,
	}

	if err := i.initDirectories(dirsToInitialize); err != nil {
		return err
	}

	var err error
	// first create directory because we use Abs and if directory does not exist
	// it will return error
	tmpDir, err = i.prepareTmpDirPath(tmpDir)
	if err != nil {
		return err
	}

	name := commandName(cmd)

	skipLock := slices.Contains(skipTmpLockCommands, name)
	if err := i.prepareStateCacheDirPath(stateDir, skipLock, tmpDir); err != nil {
		return err
	}

	releaseLock, err := acquireTmpLock(tmpDir, name, skipLock)
	if err != nil {
		return err
	}

	finalizeLogger, err := i.initLogger(cmd, tmpDir)
	if err != nil {
		releaseLock()
		return err
	}

	// exclude server command for prevent break json log in pod
	// pod uses json logs, but here json logger not initialized
	// and we got not json log string
	if skipLock && name != grpcServerCmd {
		ctx := context.Background()
		logger.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Tmp dir lock skipped because command '%s' should not acquire tmp dir", name))
	}

	runTmpCleaner := i.initTmpDirCleaner(cmd, tmpDir)

	// shutdown funcs called in reverse order
	tomb.RegisterOnShutdown("Finalize logger", finalizeLogger)
	tomb.RegisterOnShutdown("Release dhctl temporary directory lock", releaseLock)
	tomb.RegisterOnShutdown("Clear dhctl temporary directory", runTmpCleaner)
	tomb.RegisterOnShutdown("Cleanup providers from default cache", infrastructureprovider.CleanupProvidersFromDefaultCache)

	return nil
}

func (i *actionIniter) prepareStateCacheDirPath(stateCacheDir string, skipLock bool, tmpDir string) error {
	absPath, err := fs.DoAbsolutePath(stateCacheDir, true)
	if err != nil {
		return err
	}

	if fs.IsRoot(absPath) {
		return fmt.Errorf("State cache dir '%s' cannot be a root directory", stateCacheDir)
	}

	if options.DefaultTmpDir() == absPath {
		absPath = tmpDir
	}

	if !skipLock {
		if err := cache.TmpDirLockAlreadyAcquired(absPath); err != nil {
			return fmt.Errorf("Cannot use state cache dir '%s' because it can be cleaned by another instance: %v", stateCacheDir, err)
		}
	}

	i.opts.Cache.Dir = absPath
	return nil
}

func (i *actionIniter) prepareTmpDirPath(tmpDir string) (string, error) {
	absPath, err := fs.DoAbsolutePath(tmpDir, true)
	if err != nil {
		return "", err
	}

	if fs.IsRoot(absPath) {
		return "", fmt.Errorf("Tmp dir '%s' cannot be a root directory", tmpDir)
	}

	isSystem, systemDirs, err := fs.IsSystemDirOrUserHome(absPath)
	if err != nil {
		return "", err
	}

	if isSystem {
		return "", fmt.Errorf("Tmp dir '%s' cannot be a system directory or user home %v", tmpDir, systemDirs)
	}

	const breakMsg = "DHCTL can clean up this directory completely, which can break your system. Do you want to continue?"
	canceledByUser := fmt.Errorf("Operation cancelled by user")

	inSystemDirs, inSystemDirsAll := fs.IsInSystemDirs(absPath)
	if inSystemDirs {
		if !input.IsTerminal() {
			return "", fmt.Errorf("Tmp dir '%s' cannot be in system directory %v", tmpDir, inSystemDirsAll)
		}

		msg := fmt.Sprintf("The tmp dir '%s' passed to dhctl is inside a system dir '%v'. %s", tmpDir, inSystemDirsAll, breakMsg)
		if !input.NewConfirmation().WithMessage(msg).Ask() {
			return "", canceledByUser
		}
	} else {
		osTmp := os.TempDir()
		if absPath == osTmp {
			if !input.IsTerminal() {
				return "", fmt.Errorf("Tmp dir '%s' cannot be the system tmp dir %v", tmpDir, osTmp)
			}

			msg := fmt.Sprintf("The tmp dir '%s' passed to dhctl is the system tmp dir '%s'. %s", tmpDir, osTmp, breakMsg)
			if !input.NewConfirmation().WithMessage(msg).Ask() {
				return "", canceledByUser
			}
		}
	}

	i.opts.Global.TmpDir = absPath
	return absPath, nil
}

// skipTmpLockCommands never run twice at once: the gRPC server keeps its one-shot runners' tmp
// dirs below its own, and the exporter and converge-periodical run in pods whose emptyDir /tmp
// outlives an OOM-killed container, so a lock would stay behind and fail the restart.
var skipTmpLockCommands = []string{
	grpcServerCmd,
	fmt.Sprintf("%s %s", terraformGroupCmd, exporterCmd),
	autoConvergeCmd,
}

func acquireTmpLock(tmpDir, name string, skip bool) (func(), error) {
	if skip {
		return doNothingOnShutdownFunc, nil
	}
	if err := cache.TmpDirLockAlreadyAcquired(tmpDir); err != nil {
		return nil, err
	}

	release, err := cache.AcquireTmpDirLock(tmpDir, name)
	if err != nil {
		return nil, err
	}

	return release, nil
}

func (i *actionIniter) initTmpDirCleaner(cmd *cobra.Command, tmpDir string) onShutdownFunc {
	clearTmpParams := cache.ClearTmpParams{
		IsDebug:       i.opts.Global.IsDebug,
		DefaultTmpDir: options.DefaultTmpDir(),
		TmpDir:        tmpDir,
	}

	// _server is special command for running action eg bootstrap as standalone process
	// we need to remove all for this command because state will write in db
	// and do not need on fs
	if commandName(cmd) == oneShotDhctlServerCmd {
		clearTmpParams.RemoveTombStone = true
	}

	cleaner := cache.NewTmpCleaner(clearTmpParams)
	cache.SetGlobalTmpCleaner(cleaner)

	return func() {
		cache.GetGlobalTmpCleaner().Cleanup()
	}
}

type directoriesToInitialize map[string]string

func (i *actionIniter) initDirectories(dirs directoriesToInitialize) error {
	for name, dir := range dirs {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			if os.IsExist(err) {
				continue
			}

			return fmt.Errorf("Cannot create %s '%s': %w", name, dir, err)
		}
	}

	return nil
}

// forceVerboseCommands run as if -v was given: their progress is the point of running them.
var forceVerboseCommands = []string{
	"bootstrap-phase install-deckhouse",
	"bootstrap-phase create-resources",
	"bootstrap-phase base-infra",
}

var skipTeeLoggerCommands = []string{grpcServerCmd, oneShotDhctlServerCmd}
var disableLoggerCommands = []string{phaseCatalogCmd}

func (i *actionIniter) initLogger(cmd *cobra.Command, tmpDir string) (onShutdownFunc, error) {
	// Terminal logging axes:
	//   - stdoutTTY: stdout is a terminal → enable the terminal sink at all.
	//   - interactive: pinned progress bar; on only when a terminal AND not the -v linear dump.
	//   - verbose (-v): show every Info+ record on the terminal.
	// DHCTL_DEBUG (isDebug) deliberately does NOT touch the terminal — it only enriches the .log file
	// (full DEBUG is always captured there, plus shell-operator/klog internals via BindShellOperator).
	// So the terminal looks identical with or without DHCTL_DEBUG.
	verbose := i.opts.Global.ShowProgress
	stdoutTTY := input.IsTerminal() || verbose
	interactive := stdoutTTY && !verbose

	name := commandName(cmd)

	// Fully mute the logger for commands that emit pristine stdout (JSON catalogs, etc.).
	// Both sinks discard, so nothing can corrupt the command's output.
	if slices.Contains(disableLoggerCommands, name) {
		i.bindSlogRoot(cmd, telemetry.WithOTLPLogExport(logger.NewRoot(logger.Options{
			FileWriter: io.Discard,
		})))
		return doNothingOnShutdownFunc, nil
	}

	// Skip cases: build a working slog root WITHOUT a debug file. Every path still sets
	// i.slogRoot + SetDefault + binds klog/shell-op, so logger.FromContext always routes
	// to a real logger.
	if i.opts.Global.DoNotWriteDebugLogFile || slices.Contains(skipTeeLoggerCommands, name) {
		i.bindSlogRoot(cmd, telemetry.WithOTLPLogExport(logger.NewRoot(logger.Options{
			FileWriter:  os.Stdout,
			TTYWriter:   os.Stdout,
			IsTTY:       stdoutTTY,
			Interactive: interactive,
			Verbose:     verbose,
		})))
		return doNothingOnShutdownFunc, nil
	}

	logPath := i.opts.Global.DebugLogFilePath

	if logPath == "" {
		cmdStr := strings.Join(strings.Fields(name), "")
		logFile := cmdStr + "-" + time.Now().Format("20060102150405") + ".log"
		logPath = path.Join(tmpDir, logFile)
	}

	outFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	shared := newSyncWriter(outFile)

	// The slog root writes records straight to the shared file (its FileWriter), so the file
	// contains only slog records — no separate legacy tee. WithOTLPLogExport mirrors those same
	// records to OTLP when telemetry is enabled (no-op otherwise).
	i.bindSlogRoot(cmd, telemetry.WithOTLPLogExport(logger.NewRoot(logger.Options{
		FileWriter:  shared,
		TTYWriter:   os.Stdout,
		IsTTY:       stdoutTTY,
		Interactive: interactive,
		Verbose:     verbose,
	})))

	// cmd-level notice now goes through the slog root (terminal + file). Tagged for the compact
	// view: this is the pointer a user needs exactly when something went wrong, and untagged Info
	// is filtered out of the curated terminal output.
	i.slogRoot.LogAttrs(context.Background(), slog.LevelInfo, "Debug log file: "+logPath, logger.ShowInCompacted())

	i.logFile = logPath

	return func() {
		// slog writes synchronously, so closing the file is enough to flush.
		if err := shared.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close debug log file: %v\n", err)
		}
	}, nil
}

// bindSlogRoot installs root as the application slog root: stores it, makes it the slog
// default, binds klog + shell-operator to it, and carries it on the command context.
func (i *actionIniter) bindSlogRoot(cmd *cobra.Command, root *slog.Logger) {
	i.slogRoot = root

	// Make the dual-sink root the slog default, so any logger.FromContext call whose context
	// does not explicitly carry a logger (unavoidable at lifecycle/builder seams) still routes
	// to the file + TTY sinks instead of a bare stderr handler.
	slog.SetDefault(root)

	// klog and shell-operator route into the slog root.
	logger.BindKlog(root)
	logger.BindShellOperator(root, i.opts.Global.IsDebug)

	// Carry the slog root on the command context so command handlers can pull it.
	cmd.SetContext(logger.ToContext(cmd.Context(), root))
}

// commandName is the path below the root, "terraform converge-exporter": the form
// skipTmpLockCommands lists.
func commandName(cmd *cobra.Command) string {
	return strings.TrimPrefix(cmd.CommandPath(), app.AppName+" ")
}

func disableCleanupOnInterrupted(s os.Signal) {
	if !input.IsTerminal() {
		return
	}
	// disable tmp cleaning if user pass ctrl + c
	cache.GetGlobalTmpCleaner().DisableCleanup("Interrupted by signal " + s.String())
}
