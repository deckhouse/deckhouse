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
	"strings"
	"sync"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type actionIniter struct {
	logFileMutex sync.Mutex
	logFile      string
}

func newActionIniter() *actionIniter {
	return &actionIniter{}
}

func (i *actionIniter) init(c *kingpin.ParseContext) error {
	if err := i.initDirectories(); err != nil {
		return err
	}

	if err := i.initLogger(c); err != nil {
		return err
	}

	tomb.RegisterOnShutdown("Cleanup providers from default cache", func() {
		infrastructureprovider.CleanupProvidersFromDefaultCache(log.GetDefaultLogger())
	})

	return nil
}

func (i *actionIniter) initDirectories() error {
	tmpDir := app.TmpDirName
	err := os.MkdirAll(tmpDir, 0o755)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}

		return fmt.Errorf("Cannot create tmp dir %s: %w", tmpDir, err)
	}

	return nil
}

func (i *actionIniter) initLogger(c *kingpin.ParseContext) error {
	log.InitLogger(app.LoggerType)
	if app.DoNotWriteDebugLogFile {
		return nil
	}

	if c.SelectedCommand == nil {
		return nil
	}

	logPath := app.DebugLogFilePath

	if logPath == "" {
		cmdStr := strings.Join(strings.Fields(c.SelectedCommand.FullCommand()), "")
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
