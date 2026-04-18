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
	"encoding/json"
	"fmt"
)

type imagesDigests map[string]any

func GetImage(section, name string) (string, error) {
	var digests imagesDigests
	digest, err := ImagesDigestsBytes()
	if err != nil {
		return "", fmt.Errorf("could not load images digests: %w", err)
	}

	if err := json.Unmarshal(digest, &digests); err != nil {
		return "", fmt.Errorf("could not unmarshal: %w", err)
	}

	if digests[section] != nil {
		sec := digests[section].(map[string]interface{})
		img, ok := sec[name]
		if ok {
			return img.(string), nil
		}
	}

	return "", fmt.Errorf("could not find image %s in section %s", name, section)
}
