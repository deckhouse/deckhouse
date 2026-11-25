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

package minget

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
)

//go:embed all:embed
var mingetEmbeddedBinary embed.FS

const (
	mingetBinaryPathEnv = "DHCTL_MINGET_PATH"
	mingetEmbeddedPath  = "embed/minget"
)

var mingetBinaryPath = "/minget"

func Bytes(ctx context.Context) ([]byte, error) {
	binaryPath := mingetBinaryPath
	if path := os.Getenv(mingetBinaryPathEnv); path != "" {
		binaryPath = path
	}

	stat, err := os.Stat(binaryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("%s does not exist. Falling back to embedded minget", binaryPath))
		} else {
			dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf("Failed to stat %s: %v. Falling back to embedded minget", binaryPath, err), "\n"))
		}
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	if stat.IsDir() {
		dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf("%s is a directory. Falling back to embedded minget", binaryPath), "\n"))
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	file, err := os.ReadFile(binaryPath)
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, strings.TrimRight(fmt.Sprintf("Failed to open %s: %v. Falling back to embedded minget", binaryPath, err), "\n"))
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Using %s", binaryPath))
	return file, nil
}

// GzipBase64 gzips minget and base64-encodes the result, the form bb-minget-install in
// candi/bashible/lib.sh.tpl takes as mingetB64. The same script goes into the cloud-init
// user data of cloud nodes, which EC2 caps at 16384 bytes, and gzip takes a third off minget.
func GzipBase64(ctx context.Context) (string, error) {
	binary, err := Bytes(ctx)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("gzip minget: %w", err)
	}
	if _, err := zw.Write(binary); err != nil {
		return "", fmt.Errorf("gzip minget: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("gzip minget: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
