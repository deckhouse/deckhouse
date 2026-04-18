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

type ImagesDigests = map[string]map[string]any

func GetAllDigests() (ImagesDigests, error) {
	content, err := imagesDigestsContent()
	if err != nil {
		return nil, fmt.Errorf("Could not load images digests: %w", err)
	}

	var digests ImagesDigests

	if err := json.Unmarshal(content, &digests); err != nil {
		return nil, fmt.Errorf("Could not unmarshal images digests: %w", err)
	}

	return digests, nil
}

func GetImage(section, name string) (string, error) {
	digests, err := GetAllDigests()
	if err != nil {
		return "", err
	}

	sec, ok := digests[section]
	if !ok || len(sec) == 0 {
		return "", fmt.Errorf("Not found images digests section '%s' or empty", section)
	}

	imgRaw, ok := sec[name]
	if !ok {
		return "", fmt.Errorf("Not found image '%s' in section '%s'", name, section)
	}

	img, ok := imgRaw.(string)
	if !ok {
		return "", fmt.Errorf("image '%s' in section '%s' is not string. It is %T", name, section, imgRaw)
	}

	return img, nil
}
