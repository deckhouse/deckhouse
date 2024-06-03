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

package cache

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func ClearTerraformDir() {
	// do not clean tmp dir, because user may need temporary files to debug terraform
	if app.IsDebug {
		return
	}

	_ = os.RemoveAll(filepath.Join(app.TmpDirName, "tf_dhctl"))
}

func ClearTemporaryDirs() {
	// do not clean tmp dir, because user may need temporary files to debug terraform
	if app.IsDebug {
		return
	}

	_ = filepath.Walk(app.TmpDirName, func(path string, info os.FileInfo, err error) error {
		// If tmp folder doesn't exist
		if info == nil {
			return nil
		}
		if info.IsDir() {
			if path != app.TmpDirName {
				return filepath.SkipDir
			}
			return nil
		}

		// skip log files
		if strings.HasSuffix(path, ".log") {
			return nil
		}

		_ = os.Remove(path)
		return nil
	})
}
