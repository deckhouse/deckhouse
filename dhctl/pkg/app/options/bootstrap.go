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

package options

import "time"

// BootstrapOptions covers everything specific to the bootstrap flow.
type BootstrapOptions struct {
	InternalNodeIP string
	DevicePath     string

	ResourcesPath    string
	ResourcesTimeout time.Duration
	DeckhouseTimeout time.Duration

	PostBootstrapScriptTimeout time.Duration
	PostBootstrapScriptPath    string

	ForceAbortFromCache             bool
	DontUsePublicControlPlaneImages bool

	KubeadmBootstrap   bool
	MasterNodeSelector bool
}

// NewBootstrapOptions returns BootstrapOptions with the previous package-level defaults.
func NewBootstrapOptions() BootstrapOptions {
	return BootstrapOptions{
		ResourcesTimeout:           15 * time.Minute,
		DeckhouseTimeout:           15 * time.Minute,
		PostBootstrapScriptTimeout: 10 * time.Minute,
	}
}
