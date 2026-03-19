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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var version = "dev"

func versionRequested(args []string) bool {
	return len(args) > 0 && args[0] == "version"
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[rpp-get] ")
	log.SetFlags(0)
	logger := log.Default()

	if versionRequested(os.Args[1:]) {
		fmt.Println(version)
		return
	}

	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		logger.Fatal(err)
	}

	if len(cfg.packages) == 0 {
		logger.Fatalf(
			"usage: %s <%s|%s|%s> [flags] PACKAGE [PACKAGE...]",
			filepath.Base(os.Args[0]),
			modeFetch,
			modeInstall,
			modeUninstall,
		)
	}

	client := NewRppClient(cfg, logger)
	defer func() {
		if err := client.resultRecorder.close(); err != nil {
			logger.Printf("close result file: %v", err)
		}
	}()

	var runErr error
	switch cfg.mode {
	case modeFetch:
		runErr = client.FetchAll(context.Background(), cfg.packages)
	case modeInstall:
		runErr = client.InstallAll(context.Background(), cfg.packages)
	case modeUninstall:
		runErr = client.UninstallAll(context.Background(), cfg.packages)
	default:
		logger.Fatalf("unsupported mode %q", cfg.mode)
	}

	if runErr != nil {
		logger.Fatal(runErr)
	}
}
