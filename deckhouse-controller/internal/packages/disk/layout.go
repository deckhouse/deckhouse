// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package disk

import "path/filepath"

// deployedDir is the directory containing attached package instances.
const deployedDir = "deployed"

// Instance is the attach instance of a package version, name@version. The version is part of it, so
// a Candidate's image never replaces Current's. A version never contains "@": the loader takes it
// from the mount path after the first "@".
func Instance(name, version string) string {
	return name + "@" + version
}

// AttachPath returns the attachment path for a package instance.
func AttachPath(workingDir, instance string) string {
	return filepath.Join(AttachDir(workingDir), instance)
}

// AttachDir returns the directory containing attached package instances.
func AttachDir(workingDir string) string {
	return filepath.Join(workingDir, deployedDir)
}
