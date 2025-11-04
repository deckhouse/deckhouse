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

package main

import (
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type actionIniter struct {
	logFileMutex sync.Mutex
	logFile      string
}

func newActionIniter() *actionIniter {
	return &actionIniter{}
}

func getCommandName(c *kingpin.ParseContext) string {
	if c.SelectedCommand == nil {
		return ""
	}

	return c.SelectedCommand.FullCommand()
}

func (i *actionIniter) init(c *kingpin.ParseContext) error {
	tmpDir := app.TmpDirName
	dirsToInitialize := directoriesToInitialize{
		"temp dir": tmpDir,
	}

	if err := i.initDirectories(dirsToInitialize); err != nil {
		return err
	}

	if err := i.initLogger(c); err != nil {
		return err
	}

	clearTmpParams := cache.ClearTmpParams{
		IsDebug:       app.IsDebug,
		DefaultTmpDir: app.GetDefaultTmpDir(),
		TmpDir:        tmpDir,
		LoggerProvider: func() log.Logger {
			return log.GetDefaultLogger()
		},
	}

	// _server is special command for running action eg bootstrap as standalone process
	// we need to remove all for this command because state will write in db
	// and do not need on fs
	if getCommandName(c) == "_server" {
		log.DebugLn("Selected command: _server. Tombstone will be removed when temp directory remove")
		clearTmpParams.RemoveTombStone = true
	}

	tomb.RegisterOnShutdown("Cleanup providers from default cache", func() {
		infrastructureprovider.CleanupProvidersFromDefaultCache(log.GetDefaultLogger())
	})

	tomb.RegisterOnShutdown("Clear dhctl temporary directory", cache.GetClearTemporaryDirsFunc(clearTmpParams))

	return nil
}

type directoriesToInitialize map[string]string

func (i *actionIniter) initDirectories(dirs directoriesToInitialize) error {
	for name, dir := range dirs {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			if os.IsExist(err) {
				return nil
			}

			return fmt.Errorf("Cannot create %s '%s': %w", name, dir, err)
		}
	}

	return nil
}

// empty is command not passed
var skipTeeLoggerCommands = []string{"", "server", "_server"}

func (i *actionIniter) initLogger(c *kingpin.ParseContext) error {
	log.InitLogger(app.LoggerType)
	if app.DoNotWriteDebugLogFile {
		return nil
	}

	commandName := getCommandName(c)

	if slices.Contains(skipTeeLoggerCommands, commandName) {
		return nil
	}

	logPath := app.DebugLogFilePath

	if logPath == "" {
		cmdStr := strings.Join(strings.Fields(commandName), "")
		logFile := cmdStr + "-" + time.Now().Format("20060102150405") + ".log"
		logPath = path.Join(app.TmpDirName, logFile)
	}

	outFile, err := os.Create(logPath)
	if err != nil {
		return err
	}

	err = log.WrapWithTeeLogger(outFile, 1024)
	if err != nil {
		return err
	}

	log.InfoF("Debug log file: %s\n", logPath)

	tomb.RegisterOnShutdown("Finalize logger", func() {
		if err := log.FlushAndClose(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to flush and close log file: %v\n", err)
			return
		}
	})

	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	i.logFile = logPath

	return nil
}

func (i *actionIniter) getLoggerPath() string {
	i.logFileMutex.Lock()
	defer i.logFileMutex.Unlock()

	return i.logFile
}
