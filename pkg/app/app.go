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

package app

import (
	adapp "github.com/flant/addon-operator/pkg/app"
	shapp "github.com/flant/shell-operator/pkg/app"
)

// SetAddonOperatorVersion sets the version string reported by addon-operator.
func SetAddonOperatorVersion(v string) {
	adapp.Version = v
}

// SetShellOperatorVersion sets the version string reported by shell-operator.
func SetShellOperatorVersion(v string) {
	shapp.Version = v
}

// SetAppStartMessage overrides the line logged when the operator starts.
func SetAppStartMessage(msg string) {
	adapp.AppStartMessage = msg
}

// SetKubeClientFieldManager sets the field manager name for server-side apply.
// Must be set before the Kubernetes client is initialized.
func SetKubeClientFieldManager(name string) {
	shapp.KubeClientFieldManager = name
}

// SetDebugUnixSocket overrides the unix socket path for the debug endpoint.
func SetDebugUnixSocket(path string) {
	adapp.DebugUnixSocket = path
}
