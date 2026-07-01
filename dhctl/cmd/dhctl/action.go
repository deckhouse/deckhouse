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
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

type (
	onShutdownFunc func()

	registerOnShutdownFunc func(title string, action onShutdownFunc)
)

func doNothingOnShutdownFunc() {}

type actionIniterParams struct {
	stateCacheDirName string
	tmpDirName        string

	isDebug bool

	loggerType          string
	debugLogFilePath    string
	doNotWriteDebugFile bool
}

type actionIniter struct {
	logFileMutex sync.Mutex
	logFile      string

	slogRoot *slog.Logger

	opts               *options.Options
	params             *actionIniterParams
	registerOnShutdown registerOnShutdownFunc
}

func newActionIniter(opts *options.Options) *actionIniter {
	return &actionIniter{opts: opts}
}

// Logger returns the application slog root, or a stderr fallback if not yet initialized.
func (i *actionIniter) Logger() *slog.Logger {
	if i.slogRoot != nil {
		return i.slogRoot
	}
	return logger.NewRoot(logger.Options{FileWriter: os.Stderr})
}

func (i *actionIniter) setParams(params actionIniterParams) *actionIniter {
	i.params = new(params)
	return i
}

func (i *actionIniter) setRegisterOnShutdown(f registerOnShutdownFunc) *actionIniter {
	i.registerOnShutdown = f
	return i
}

func (i *actionIniter) init(c *kingpin.ParseContext) error {
	if i.params == nil {
		return fmt.Errorf("Internal error: action initer not initialized")
	}

	if i.registerOnShutdown == nil {
		return fmt.Errorf("Internal error: action initer not initialized. The register-on-shutdown callback was not passed")
	}

	tmpDir := i.params.tmpDirName
	if tmpDir == "" {
		return fmt.Errorf("Internal error: action initer not initialized. Tmp dir is empty")
	}

	stateDir := i.params.stateCacheDirName
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

	if err := i.prepareStateCacheDirPath(stateDir, c, tmpDir); err != nil {
		return err
	}

	tmpDirLockResult, err := i.checkAndAcquireTmpLock(c, tmpDir)
	if err != nil {
		return err
	}

	finalizeLogger, err := i.initLogger(c, tmpDir)
	if err != nil {
		tmpDirLockResult.onShutdown()
		return err
	}

	// exclude server command for prevent break json log in pod
	// pod uses json logs, but here json logger not initialized
	// and we got not json log string
	if tmpDirLockResult.skipped && tmpDirLockResult.skippedBy != grpcServerCmd {
		ctx := context.Background()
		logger.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Tmp dir lock skipped because command '%s' should not acquire tmp dir", tmpDirLockResult.skippedBy))
	}

	runTmpCleaner := i.initTmpDirCleaner(c, tmpDir)

	// shutdown funcs called in reverse order

	i.registerOnShutdown("Finalize logger", finalizeLogger)

	i.registerOnShutdown("Release dhctl temporary directory lock", tmpDirLockResult.onShutdown)

	i.registerOnShutdown("Clear dhctl temporary directory", runTmpCleaner)

	i.registerOnShutdown("Cleanup providers from default cache", func() {
		infrastructureprovider.CleanupProvidersFromDefaultCache()
	})

	return nil
}

func (i *actionIniter) prepareStateCacheDirPath(stateCacheDir string, c *kingpin.ParseContext, tmpDir string) error {
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

	if skipCheckAcquire, _ := i.skipCheckAcquireTmpLock(c); !skipCheckAcquire {
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

var skipTmpLockCommands = []string{
	// empty is command not passed
	"",
	grpcServerCmd,
	fmt.Sprintf("%s %s", terraformGroupCmd, exporterCmd),
	autoConvergeCmd,
}

func (i *actionIniter) skipCheckAcquireTmpLock(c *kingpin.ParseContext) (bool, string) {
	cmdName := getCommandName(c)
	// do not lock for grpc server because for singleshot dhctl runner we create
	// tmp dir in sub directory of server
	// exporter and autoconverger run in pods
	// for pods we are using empty dir for /tmp
	// when container is killed (for example by OOM) empty dir was not cleaned
	// and we got lock error on container restart
	// we can safe skip tmp dir lock because we cannot get sutuation
	// when multiple commands run in parallel
	return slices.Contains(skipTmpLockCommands, cmdName), cmdName
}

type tmpLockAcquireResult struct {
	onShutdown onShutdownFunc
	skipped    bool
	skippedBy  string
}

func (i *actionIniter) checkAndAcquireTmpLock(c *kingpin.ParseContext, tmpDir string) (*tmpLockAcquireResult, error) {
	skipAcquire, cmdName := i.skipCheckAcquireTmpLock(c)
	if skipAcquire {
		return &tmpLockAcquireResult{
			onShutdown: doNothingOnShutdownFunc,
			skipped:    true,
			skippedBy:  cmdName,
		}, nil
	}

	if err := cache.TmpDirLockAlreadyAcquired(tmpDir); err != nil {
		return nil, err
	}

	releaseLock, err := cache.AcquireTmpDirLock(tmpDir, cmdName)
	if err != nil {
		return nil, err
	}

	return &tmpLockAcquireResult{
		onShutdown: func() {
			releaseLock()
		},
		skipped: false,
	}, nil
}

func (i *actionIniter) initTmpDirCleaner(c *kingpin.ParseContext, tmpDir string) onShutdownFunc {
	clearTmpParams := cache.ClearTmpParams{
		IsDebug:          i.params.isDebug,
		DefaultTmpDir:    options.DefaultTmpDir(),
		DownloadCacheDir: i.opts.Global.DownloadCacheDir,
		TmpDir:           tmpDir,
	}

	// _server is special command for running action eg bootstrap as standalone process
	// we need to remove all for this command because state will write in db
	// and do not need on fs
	if getCommandName(c) == oneShotDhctlServerCmd {
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

// empty is command not passed
var skipTeeLoggerCommands = []string{"", grpcServerCmd, oneShotDhctlServerCmd}

func (i *actionIniter) initLogger(c *kingpin.ParseContext, tmpDir string) (onShutdownFunc, error) {
	// Terminal logging axes:
	//   - stdoutTTY: stdout is a terminal → enable the terminal sink at all.
	//   - interactive: pinned progress bar; on only when a terminal AND not the -v linear dump.
	//   - verbose (-v): show every Info+ record on the terminal.
	// DHCTL_DEBUG (isDebug) deliberately does NOT touch the terminal — it only enriches the .log file
	// (full DEBUG is always captured there, plus shell-operator/klog internals via BindShellOperator).
	// So the terminal looks identical with or without DHCTL_DEBUG.
	stdoutTTY := input.IsTerminal()
	interactive := stdoutTTY && !i.opts.Global.ShowProgress
	verbose := i.opts.Global.ShowProgress

	commandName := getCommandName(c)

	// Skip cases: build a working slog root WITHOUT a debug file. Every path still sets
	// i.slogRoot + SetDefault + binds klog/shell-op, so logger.FromContext always routes
	// to a real logger.
	if i.params.doNotWriteDebugFile || slices.Contains(skipTeeLoggerCommands, commandName) {
		i.bindSlogRoot(c, logger.NewRoot(logger.Options{
			FileWriter:  os.Stdout,
			TTYWriter:   os.Stdout,
			IsTTY:       stdoutTTY,
			Interactive: interactive,
			Verbose:     verbose,
		}))
		return doNothingOnShutdownFunc, nil
	}

	logPath := i.params.debugLogFilePath

	if logPath == "" {
		cmdStr := strings.Join(strings.Fields(commandName), "")
		logFile := cmdStr + "-" + time.Now().Format("20060102150405") + ".log"
		logPath = path.Join(tmpDir, logFile)
	}

	outFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	shared := newSyncWriter(outFile)

	// The slog root writes records straight to the shared file (its FileWriter), so the file
	// contains only slog records — no separate legacy tee.
	i.bindSlogRoot(c, logger.NewRoot(logger.Options{
		FileWriter:  shared,
		TTYWriter:   os.Stdout,
		IsTTY:       stdoutTTY,
		Interactive: interactive,
		Verbose:     verbose,
	}))

	// cmd-level notice now goes through the slog root (terminal + file).
	i.slogRoot.Info("Debug log file: " + logPath)

	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	i.logFile = logPath

	return func() {
		// slog writes synchronously, so closing the file is enough to flush.
		if err := shared.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close debug log file: %v\n", err)
		}
	}, nil
}

// bindSlogRoot installs root as the application slog root: stores it, makes it the slog
// default, binds klog + shell-operator to it, and carries it on the kingpin context.
func (i *actionIniter) bindSlogRoot(c *kingpin.ParseContext, root *slog.Logger) {
	i.slogRoot = root

	// Make the dual-sink root the slog default, so any logger.FromContext call whose context
	// does not explicitly carry a logger (unavoidable at lifecycle/builder seams) still routes
	// to the file + TTY sinks instead of a bare stderr handler.
	slog.SetDefault(root)

	// klog and shell-operator route into the slog root.
	logger.BindKlog(root)
	logger.BindShellOperator(root, i.params.isDebug)

	// Carry the slog root on the kingpin context so command handlers can pull it.
	kpcontext.SetContextToParseContext(logger.ToContext(kpcontext.ExtractContext(c), root), c)
}

func (i *actionIniter) getLoggerPath() string {
	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	return i.logFile
}

func getCommandName(c *kingpin.ParseContext) string {
	if c.SelectedCommand == nil {
		return ""
	}
	// todo be carefully during migration to cobra
	// we use full command for check command in skipCheckAcquireTmpLock
	// and we have "terraform converge-exporter" command
	// need to check that cobra returns same result or fix cmd name in skipCheckAcquireTmpLock
	return c.SelectedCommand.FullCommand()
}

func disableCleanupOnInterrupted(s os.Signal) {
	if !input.IsTerminal() {
		return
	}
	// disable tmp cleaning if user pass ctrl + c
	cache.GetGlobalTmpCleaner().DisableCleanup("Interrupted by signal " + s.String())
}
