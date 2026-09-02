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

var Version string

// SetDeckhouseVersion sets the version string reported by deckhouse-controller.
func SetDeckhouseVersion(v string) {
	Version = v
}

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

// Admission carries the settings the validating webhook server starts with.
type Admission struct {
	ListenPort string
	CertsDir   string
}

// TakeOverAdmissionServer hands the admission settings to the caller and keeps
// addon-operator's own admission server down, since both would bind the same
// port. Reports false when there is nothing to serve: the dhctl bootstrap
// incarnation mounts no certificates.
func TakeOverAdmissionServer() (Admission, bool) {
	if !adapp.AdmissionServerEnabled {
		return Admission{}, false
	}

	adapp.AdmissionServerEnabled = false

	return Admission{
		ListenPort: adapp.AdmissionServerListenPort,
		CertsDir:   adapp.AdmissionServerCertsDir,
	}, true
}
