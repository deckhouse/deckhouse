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

// Package containerd reconciles the containerd registry hosts drop-in
// directory (/etc/containerd/registry.d) to a desired state. It writes only
// drop-in files; it never edits /etc/containerd/config.toml.
package containerd

// DesiredState is the full set of registry.d drop-ins the agent wants present.
// Hosts is keyed by the managed registry host that containerd resolves
// (e.g. "registry.d8-system.svc:5001").
type DesiredState struct {
	Hosts map[string]HostConfig
}

// HostConfig is the drop-in for a single managed registry host.
type HostConfig struct {
	// Server is the hosts.toml "server" value — the managed registry host
	// itself (e.g. "registry.d8-system.svc:5001").
	Server string
	// Entries are the endpoints containerd tries, in order. The first is the
	// preferred route (the local agent); later entries are failover targets.
	Entries []HostEntry
}

// HostEntry is one [host."URL"] table in hosts.toml.
type HostEntry struct {
	// URL is the endpoint including scheme, e.g. "https://127.0.0.1:5001".
	URL string
	// Capabilities is the containerd capability list, e.g. ["pull","resolve"].
	Capabilities []string
	// CA is a PEM CA bundle for this endpoint; empty means none.
	CA string
	// SkipVerify disables TLS verification (used for http/insecure endpoints).
	SkipVerify bool
	// Auth is base64("user:password") written under [host."URL".auth]; empty
	// means no auth.
	Auth string
}
