/*
Copyright 2026 Flant JSC

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

// Package config maps a parsed RegistryConfig resource into the agent's
// containerd desired state and proxy routes.
package config

// RegistryConfig is the parsed spec of the RegistryConfig custom resource.
type RegistryConfig struct {
	Registries []RegistryEntry
	Auth       AuthSpec
}

// Source constants identify the origin of a RegistryEntry.
const (
	SourcePrimary      = "Primary"
	SourceAdditional   = "Additional"
	SourceModuleSource = "ModuleSource"
)

// RegistryEntry is one intercepted registry.
type RegistryEntry struct {
	Host     string
	Source   string // Primary | Additional | ModuleSource
	Upstream *UpstreamSpec
	Cache    *CacheSpec
}

// UpstreamSpec is a real upstream registry.
type UpstreamSpec struct {
	Host        string
	Path        string
	Scheme      string // HTTP | HTTPS
	CA          string
	Credentials *Credentials
}

// Credentials are upstream registry credentials.
type Credentials struct {
	Username  string
	Password  string
	DockerCfg string
}

// CacheSpec is the caching policy for an entry.
type CacheSpec struct {
	Enabled bool
}

// AuthSpec is the local auth config.
type AuthSpec struct {
	Users []UserSpec
}

// UserSpec is a local registry user.
type UserSpec struct {
	Name string
	Role string // ReadOnly | ReadWrite
}
