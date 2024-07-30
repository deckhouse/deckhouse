// Copyright 2023 Flant JSC
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

package mirror

import (
	"fmt"
	"io"

	"go.cypherpunks.ru/gogost/v5/gost34112012256"
)

func CalculateBlobGostDigest(blobStream io.Reader) (string, error) {
	stribog := gost34112012256.New()
	if _, err := io.Copy(stribog, blobStream); err != nil {
		return "", fmt.Errorf("digest blob: %w", err)
	}

	return fmt.Sprintf("%x", stribog.Sum(nil)), nil
}
