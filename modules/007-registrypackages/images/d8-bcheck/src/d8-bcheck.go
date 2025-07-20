/*
Copyright 2023 Flant JSC

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
	"errors"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/deckhouse/delivery-kit-sdk/pkg/signature/elf"
	"github.com/deckhouse/delivery-kit-sdk/pkg/signature/elf/inhouse"
	"github.com/deckhouse/rootca"
)

const (
	sleep        = 30 * time.Second
	observedPath = "/opt/deckhouse/bin"
	title        = "deckhouse-binary-checker checks dechouse binaries signature in the " + observedPath
)

func FilePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// check symlink
		if info.Mode()&os.ModeSymlink != 0 {
			path, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// due to symlinks we can have duplicated files
	s := make(map[string]struct{}, len(files))
	for _, f := range files {
		s[f] = struct{}{}
	}

	return slices.Collect(maps.Keys(s)), nil
}

func verifyBinaries(ctx context.Context) {
	for {
		files, err := FilePathWalkDir(observedPath)
		if err != nil {
			panic(err)
		}

		for _, file := range files {
			select {
			case <-ctx.Done():
				return
			default:
				slog.Info("processing", "file", file)
				err := inhouse.Verify(ctx, string(rootca.RootCA), file)
				if errors.Is(err, elf.ErrNotELF) {
					continue
				}
				if err != nil {
					if err := generateSyscall(file); err != nil {
						slog.Error("failed to generate syscall", "file", file, "error", err)
					}
					slog.Error("failed to verify", "file", file, "error", err)
				}
			}
		}
		time.Sleep(sleep)
	}
}

// generates syscall event to process via falco rules
func generateSyscall(filePath string) error {
	slog.Error("generate syscall event", "file", filePath)
	f, err := os.OpenFile(filePath, os.O_RDONLY|os.O_SYNC, 0644)
	defer f.Close()
	return err
}

func main() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info(title)

	go verifyBinaries(ctx)
	// Listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Block until a signal is received
	<-sigChan
	slog.Info("Received OS signal, initiating graceful shutdown")
}
