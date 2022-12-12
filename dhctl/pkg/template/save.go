// Copyright 2021 Flant JSC
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

package template

import (
	"fmt"
	"os"
	"path"
)

func SaveRenderedToDir(renderedTpls []RenderedTemplate, dirToSave string) error {
	if err := os.MkdirAll(dirToSave, os.ModePerm); err != nil {
		return fmt.Errorf("creating rendered templates dir: %v", err)
	}

	for _, rendered := range renderedTpls {
		filename := path.Join(dirToSave, rendered.FileName)
		err := os.WriteFile(filename, rendered.Content.Bytes(), bundlePermissions)
		if err != nil {
			return fmt.Errorf("saving rendered file %s: %v", rendered.FileName, err)
		}
	}

	return nil
}
