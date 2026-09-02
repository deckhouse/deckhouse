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
	"time"

	adapp "github.com/flant/addon-operator/pkg/app"
)

// The getters below expose addon-operator's mutable package globals. They stay
// functions on purpose: the globals are read at runtime after ApplyConfig has
// merged env vars and CLI flags, so a plain var copy would freeze the value at
// init time, before the config is resolved.

// ListenAddress is the address the operator serves Prometheus metrics on.
func ListenAddress() string {
	return adapp.ListenAddress
}

// StrictModeEnabled reports whether a missing values.yaml is a fatal error.
func StrictModeEnabled() bool {
	return adapp.StrictModeEnabled
}

// KubeContext is the kubeconfig context name to use.
func KubeContext() string {
	return adapp.KubeContext
}

// KubeConfig is the path to the kubeconfig file.
func KubeConfig() string {
	return adapp.KubeConfig
}

// KubeClientQPS is the QPS limit for the main Kubernetes client.
func KubeClientQPS() float32 {
	return adapp.KubeClientQPS
}

// KubeClientBurst is the burst limit for the main Kubernetes client.
func KubeClientBurst() int {
	return adapp.KubeClientBurst
}

// ObjectPatcherKubeClientQPS is the QPS limit for the object patcher client.
func ObjectPatcherKubeClientQPS() float32 {
	return adapp.ObjectPatcherKubeClientQPS
}

// ObjectPatcherKubeClientBurst is the burst limit for the object patcher client.
func ObjectPatcherKubeClientBurst() int {
	return adapp.ObjectPatcherKubeClientBurst
}

// ObjectPatcherKubeClientTimeout is the request timeout for the object patcher client.
func ObjectPatcherKubeClientTimeout() time.Duration {
	return adapp.ObjectPatcherKubeClientTimeout
}

// HelmMonitorKubeClientQPS is the QPS limit for the Helm resources monitor client.
func HelmMonitorKubeClientQPS() float32 {
	return adapp.HelmMonitorKubeClientQps
}

// HelmMonitorKubeClientBurst is the burst limit for the Helm resources monitor client.
func HelmMonitorKubeClientBurst() int {
	return adapp.HelmMonitorKubeClientBurst
}

// DebugKeepTmpFiles reports whether temporary hook files are kept for debugging.
func DebugKeepTmpFiles() bool {
	return adapp.DebugKeepTmpFiles
}

// LogProxyHookJSON reports whether hook stdout/stderr JSON logging is proxied.
func LogProxyHookJSON() bool {
	return adapp.LogProxyHookJSON
}

// DebugUnixSocket is the path to the debug endpoint unix socket.
func DebugUnixSocket() string {
	return adapp.DebugUnixSocket
}
