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

	"rpp-get/rpp"
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

	if err := run(context.Background(), logger); err != nil {
		logger.Fatal(err)
	}
}

func run(ctx context.Context, logger *log.Logger) error {
	cfg, err := loadConfig(ctx, os.Args[1:])
	if err != nil {
		return err
	}

	if len(cfg.packages) == 0 {
		return fmt.Errorf(
			"usage: %s <%s|%s|%s> [flags] PACKAGE [PACKAGE...]",
			filepath.Base(os.Args[0]),
			modeFetch,
			modeInstall,
			modeUninstall,
		)
	}

	recorder, err := rpp.NewResultRecorder(cfg.resultPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := recorder.Close(); err != nil {
			logger.Printf("close result file: %v", err)
		}
	}()

	rppCfg := rpp.Config{
		Endpoints:      cfg.endpoints,
		Token:          cfg.token,
		Repository:     cfg.rppRepository,
		Path:           cfg.rppPath,
		Retries:        cfg.retries,
		RetryDelay:     cfg.retryDelay,
		Force:          cfg.force,
		TempDir:        cfg.tempDir,
		InstalledStore: cfg.installedStore,
	}

	client := rpp.NewClient(rppCfg, logger, recorder)

	switch cfg.mode {
	case modeFetch:
		return client.FetchAll(ctx, cfg.packages)
	case modeInstall:
		return client.InstallAll(ctx, cfg.packages)
	case modeUninstall:
		return client.UninstallAll(ctx, cfg.packages)
	default:
		panic(fmt.Sprintf("unsupported mode %q", cfg.mode))
	}
}
