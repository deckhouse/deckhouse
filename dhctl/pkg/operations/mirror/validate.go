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

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/validate"
)

type ValidationMode string

const (
	NoValidation   ValidationMode = "off"
	FastValidation ValidationMode = "fast"
	FullValidation ValidationMode = "full"
)

func ValidateLayouts(layouts []layout.Path, validationMode ValidationMode) error {
	if validationMode == NoValidation {
		return nil
	}

	opts := []validate.Option{}
	if validationMode == FastValidation {
		opts = append(opts, validate.Fast)
	}

	for _, path := range layouts {
		index, err := path.ImageIndex()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err = validate.Index(index, opts...); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}
