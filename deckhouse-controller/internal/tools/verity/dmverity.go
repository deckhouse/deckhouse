/*
Copyright 2025 Flant JSC

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

package verity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// this is util to manage dm-verity
	verityCommand = "veritysetup"
	// nolint: unused
	dmTemplate = "/dev/mapper/%s"

	// formatArg computes the Merkle tree
	formatArg = "format"
	// verifyArg verifies and gets the root hash
	verifyArg = "verify"
	// openArg creates device mapper
	openArg = "open"
	// closeArg closes device mapper
	closeArg = "close"

	blockSizeArg = "--data-block-size=4096"
	hashSizeArg  = "--hash-block-size=4096"

	// magic salt
	magicVeritySalt = "dc0f616e4bf75776061d5ffb7a6f45e1313b7cc86f3aa49b68de4f6d187bad2b"

	saltArg = "--salt=" + magicVeritySalt
)

// CreateMapper creates device mapper for the erofs image.
// It creates two loop devices and attach image and hash file for them.
// Equivalent shell command:
// veritysetup open <imagePath> <module> <hashPath> <hash>
func CreateMapper(ctx context.Context, imagePath, hash string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateMapper")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))

	return waitUntilMapperCreated(ctx, imagePath, hash)
}

// waitUntilMapperCreated waits until /dev/mapper/<module> appears.
// veritysetup open can return loop attaching error, it happens due to kernel race, so retry until ready
func waitUntilMapperCreated(ctx context.Context, imagePath, hash string) error {
	// magic numbers
	interval := 200 * time.Millisecond
	timeout := 3 * time.Second
	var lastErr error

	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		hashPath := fmt.Sprintf("%s.verity", imagePath)
		module := filepath.Base(filepath.Dir(imagePath))

		args := []string{
			openArg,
			imagePath,
			module,
			hashPath,
			hash,
		}

		// veritysetup open <imagePath> <module> <hashPath> <hash>
		cmd := exec.CommandContext(ctx, verityCommand, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("veritysetup open: %w (last output: %s)", err, string(out))
			return false, nil
		}

		return true, nil
	})

	if lastErr != nil {
		return lastErr
	}

	return err
}

// CloseMapper closes device mapper for the module
// Equivalent shell command:
// veritysetup close <module>
func CloseMapper(ctx context.Context, module string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CloseMapper")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))

	args := []string{
		closeArg,

		module,
	}

	// veritysetup close <module>
	cmd := exec.CommandContext(ctx, verityCommand, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		// mapper not found
		if strings.Contains(string(output), "not active") {
			return nil
		}

		return fmt.Errorf("veritysetup close: %w (output: %s)", err, string(output))
	}

	return nil
}

// CreateImageHash computes hash from the image by veritysetup format
// Equivalent shell command:
// veritysetup format --data-block-size=4096 --hash-block-size=4096 --salt=<salt> <imagePath> <hashPath>
func CreateImageHash(ctx context.Context, imagePath string) (string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateImageHash")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))

	stat, err := os.Stat(imagePath)
	if err != nil {
		return "", fmt.Errorf("stat erofs image: %w", err)
	}

	size := stat.Size()

	// verity partition (LABEL=VERITY), should require 8-10% the size of Root.
	hashSize := int64(float64(size) * 0.1)
	if hashSize < 4*1024*1024 {
		hashSize = 4 * 1024 * 1024
	}

	hashPath := fmt.Sprintf("%s.verity", imagePath)
	file, err := os.Create(hashPath)
	if err != nil {
		return "", fmt.Errorf("create hash image: %w", err)
	}
	defer file.Close()

	if err = file.Truncate(hashSize); err != nil {
		return "", fmt.Errorf("truncate hash image: %w", err)
	}

	hash, err := veritySetupFormat(ctx, imagePath, hashPath)
	if err != nil {
		return "", err
	}

	if len(hash) == 0 {
		return "", errors.New("empty hash")
	}

	return hash, nil
}

// VerifyImage performs verification of the erofs image against its hash tree using veritysetup.
// Equivalent shell command:
// veritysetup verify <imagePath> <hashPath> <root_hash>
func VerifyImage(ctx context.Context, imagePath, rootHash string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "Verify")
	defer span.End()

	span.SetAttributes(attribute.String("imagePath", imagePath))

	if strings.TrimSpace(rootHash) == "" {
		return errors.New("empty root hash")
	}

	hashPath := fmt.Sprintf("%s.verity", imagePath)
	if _, err := os.Stat(hashPath); err != nil {
		return fmt.Errorf("stat image hash file: %w", err)
	}

	args := []string{
		verifyArg,

		imagePath,
		hashPath,
		rootHash,
	}

	// veritysetup verify <imagePath> <hashPath> <root_hash>
	cmd := exec.CommandContext(ctx, verityCommand, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("veritysetup verify: %w (output: %s)", err, string(output))
	}

	return nil
}

// veritySetupFormat calls veritysetup utils to create image verity file
func veritySetupFormat(ctx context.Context, imagePath, hashPath string) (string, error) {
	args := []string{
		formatArg,
		blockSizeArg,
		hashSizeArg,
		saltArg,

		imagePath,
		hashPath,
	}

	// veritysetup format --data-block-size=4096 --hash-block-size=4096 --salt=<salt> <imagePath> <hashPath>
	cmd := exec.CommandContext(ctx, verityCommand, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("veritysetup format: %w (output: %s)", err, string(output))
	}

	return extractRootHash(string(output)), nil
}

func extractRootHash(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Root hash:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Root hash:"))
		}
	}

	return ""
}
