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

package digests

import (
	"embed"
	"errors"
	"os"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

//go:embed images_digests.json
var imagesDigestsEmbeddedJSON embed.FS

var imagesDigestsJSON = "/deckhouse/candi/images_digests.json"

func ImagesDigestsBytes() ([]byte, error) {
	stat, err := os.Stat(imagesDigestsJSON)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.InfoF("%s not exists. Fallback to embedded images_digests.json", imagesDigestsJSON, err)
		} else {
			log.WarnF("Failed to stat %s: %w. Fallback to embedded images_digests.json", imagesDigestsJSON, err)
		}
		return imagesDigestsEmbeddedJSON.ReadFile("images_digests.json")
	}

	if stat.IsDir() {
		log.WarnF("%s stats as directory. Fallback to embedded images_digests.json", imagesDigestsJSON)
		return imagesDigestsEmbeddedJSON.ReadFile("images_digests.json")
	}

	file, err := os.ReadFile(imagesDigestsJSON)
	if err != nil {
		log.WarnF("Failed to open %s. Fallback to embedded images_digests.json", imagesDigestsJSON)
		return imagesDigestsEmbeddedJSON.ReadFile("images_digests.json")
	}

	log.DebugF("Using %s\n", imagesDigestsJSON)
	return file, nil
}
