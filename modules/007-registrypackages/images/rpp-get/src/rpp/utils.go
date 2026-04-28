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

package rpp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func defaultFetchedStore(tempDir string) string {
	return filepath.Join(tempDir, "registrypackages")
}

func parsePackageWithDigest(value string) (string, string, error) {
	pkg, digest, ok := strings.Cut(strings.TrimSpace(value), ":")
	pkg = strings.TrimSpace(pkg)
	digest = strings.TrimSpace(digest)
	if !ok || pkg == "" || digest == "" || !strings.Contains(digest, ":") {
		return "", "", fmt.Errorf("invalid PACKAGE_WITH_DIGEST %q, expected package:sha256:<digest>", value)
	}

	return pkg, digest, nil
}

func formatSize(size int64) string {
	if size < 0 {
		return "unknown size"
	}

	return fmt.Sprintf("%d bytes", size)
}

func writeResponseBody(outputPath string, body io.Reader) error {
	tmpPath := outputPath + ".part"
	defer os.Remove(tmpPath)

	if err := writeFile(tmpPath, body); err != nil {
		return err
	}

	return os.Rename(tmpPath, outputPath)
}

func writeFile(path string, body io.Reader) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	_, err = io.Copy(file, body)
	return err
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runParallel[T any](ctx context.Context, items []T, workers int, action func(context.Context, T) error) error {
	if len(items) == 0 {
		return nil
	}

	workers = max(1, workers)
	workers = min(workers, len(items))

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	jobs := make(chan T)

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for item := range jobs {
				if ctx.Err() != nil {
					return
				}

				if err := action(ctx, item); err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		}()
	}

	for _, item := range items {
		select {
		case jobs <- item:
		case <-ctx.Done():
			mu.Lock()
			errs = append(errs, ctx.Err())
			mu.Unlock()
			close(jobs)
			wg.Wait()
			return errors.Join(errs...)
		}
	}

	close(jobs)
	wg.Wait()

	return errors.Join(errs...)
}

func statPath(path string) (os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return info, true, nil
}
