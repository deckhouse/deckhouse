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
	"embed"
	"errors"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

//go:embed all:embed
var mingetEmbeddedBinary embed.FS

const (
	mingetBinaryPathEnv = "DHCTL_MINGET_PATH"
	mingetEmbeddedPath  = "embed/minget"
)

var mingetBinaryPath = "/minget"

func Bytes() ([]byte, error) {
	binaryPath := mingetBinaryPath
	if path := os.Getenv(mingetBinaryPathEnv); path != "" {
		binaryPath = path
	}

	stat, err := os.Stat(binaryPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.InfoF("%s not exists. Fallback to embedded minget", binaryPath)
		} else {
			log.WarnF("Failed to stat %s: %v. Fallback to embedded minget", binaryPath, err)
		}
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	if stat.IsDir() {
		log.WarnF("%s stats as directory. Fallback to embedded minget", binaryPath)
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	file, err := os.ReadFile(binaryPath)
	if err != nil {
		log.WarnF("Failed to open %s: %v. Fallback to embedded minget", binaryPath, err)
		return mingetEmbeddedBinary.ReadFile(mingetEmbeddedPath)
	}

	log.DebugF("Using %s\n", binaryPath)
	return file, nil
}
