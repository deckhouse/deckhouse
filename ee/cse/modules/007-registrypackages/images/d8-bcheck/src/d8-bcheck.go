/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			// fix relative path
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}

			target, err = filepath.EvalSymlinks(target)
			if err != nil {
				return err
			}
			path = target
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
				err := inhouse.Verify(ctx, []string{rootca.RootCABase64}, file)
				if errors.Is(err, elf.ErrNotELF) {
					continue
				}
				if err != nil {
					if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
						slog.Warn("skip non-signature error", "file", file, "error", err)
						continue
					}

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
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0)
	f.Close()
	return err
}

// verifySingle runs a one-shot verification of a single file with rich diagnostics in logs.
func verifySingle(ctx context.Context, target string) error {
	start := time.Now()
	slog.Info("single-file verification: starting", "file", target)

	info, _ := os.Stat(target)
	if info.IsDir() {
		slog.Error("path is a directory, expected a regular file", "path", target)
		return errors.New("path is a directory")
	}
	slog.Info("verifier: inhouse.Verify", "root_ca_present", rootca.RootCABase64 != "")
	slog.Info("calling verifier", "file", target)
	err := inhouse.Verify(ctx, []string{rootca.RootCABase64}, target)

	switch {
	case errors.Is(err, elf.ErrNotELF):
		slog.Info(
			"verification skipped: not an ELF binary",
			"file", target,
			"elapsed", time.Since(start).String(),
		)
		return nil

	case err != nil:
		slog.Error("verification failed", "file", target, "error", err)
		slog.Info(
			"single-file verification: finished",
			"file", target,
			"result", "failure",
			"elapsed", time.Since(start).String(),
		)
		return err

	default:
		slog.Info("verification succeeded", "file", target)
		slog.Info(
			"single-file verification: finished",
			"file", target,
			"result", "success",
			"elapsed", time.Since(start).String(),
		)
		return nil
	}
}

func main() {
	ctx, cancelFunc := context.WithCancel(context.Background())

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info(title)

	if len(os.Args) > 1 {
		if err := verifySingle(ctx, os.Args[1]); err != nil {
			cancelFunc()
			os.Exit(1)
		}
		cancelFunc()
		return
	}

	go verifyBinaries(ctx)
	// Listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Block until a signal is received
	<-sigChan
	slog.Info("Received OS signal, initiating graceful shutdown")
	cancelFunc()
}
