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
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"

	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
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
