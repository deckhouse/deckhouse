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
	"time"
)

const (
	DownloadedModulesDir = "DOWNLOADED_MODULES_DIR"
	EmbeddedModulesDir   = "EMBEDDED_MODULES_DIR"

	// DocumentationBuildTimeout caps a single upload+build round-trip from the
	// module-documentation controller to a docs-builder. Every build triggers a
	// full-site Hugo rebuild serialized across all modules, so this must exceed
	// the worst-case build time; too low a value cancels healthy builds and
	// churns the builder. A Go duration string (e.g. "120s", "2m").
	DocumentationBuildTimeout = "DOCUMENTATION_BUILD_TIMEOUT"

	defaultEmbeddedModulesDir        = "/deckhouse/modules"
	defaultDocumentationBuildTimeout = 120 * time.Second
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

// GetDocumentationBuildTimeout is the per-request timeout the module-documentation
// controller applies to docs-builder upload/build calls. It parses
// DOCUMENTATION_BUILD_TIMEOUT as a Go duration (per time.ParseDuration) and falls
// back to 120s when the variable is unset, empty, or unparseable.
func GetDocumentationBuildTimeout() time.Duration {
	value := os.Getenv(DocumentationBuildTimeout)
	if len(value) == 0 {
		return defaultDocumentationBuildTimeout
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultDocumentationBuildTimeout
	}
	return parsed
}
