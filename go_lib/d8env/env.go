/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package d8env

import (
	"os"
)

const (
	DownloadedModulesDir = "DOWNLOADED_MODULES_DIR"
	EmbeddedModulesDir   = "EMBEDDED_MODULES_DIR"

	defaultEmbeddedModulesDir = "/deckhouse/modules"
)

func GetDownloadedModulesDir() string {
	value := os.Getenv(DownloadedModulesDir)
	if len(value) != 0 {
		return value
	}
	return os.Getenv("EXTERNAL_MODULES_DIR")
}

// GetEmbeddedModulesDir returns the directory where embedded (built-in) modules
// are shipped within the Deckhouse image. It is the first entry of the module
// search path, so a module present here always wins over a downloaded one.
func GetEmbeddedModulesDir() string {
	value := os.Getenv(EmbeddedModulesDir)
	if len(value) != 0 {
		return value
	}
	return defaultEmbeddedModulesDir
}
