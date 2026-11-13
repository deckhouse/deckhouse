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

package ondemand

import "context"

// RenderListFunc renders the page of the list of modules of every language
// from the cards in data, using workDir for temporary files.
type RenderListFunc func(ctx context.Context, data []byte, workDir string) (map[string][]byte, error)

// Listed is a module on the list of modules, as targets.json keeps it: the
// list shows every module with a registry source, enabled or not, also
// before its target is published.
type Listed struct {
	Module  string `json:"module"`
	Enabled bool   `json:"enabled,omitempty"`
	// Stage is the lifecycle stage the cluster knows of the module, shown
	// until the target brings its own.
	Stage string `json:"stage,omitempty"`
	// NoDocs marks a module with nothing to read: no version to render, or
	// the version of its target has no documentation pages.
	NoDocs bool `json:"noDocs,omitempty"`
	// Failed marks a module whose target failed for a lasting reason: its
	// card does not promise the details, they wait for the retry.
	Failed bool `json:"failed,omitempty"`
}

// ModuleMenu is what the version menu of a module page shows.
type ModuleMenu struct {
	// Enabled marks a module enabled in the cluster: its stable channel is
	// the installed version.
	Enabled bool `json:"enabled"`
	// Channels are the served release channels and their versions.
	Channels Channels `json:"channels"`
}
