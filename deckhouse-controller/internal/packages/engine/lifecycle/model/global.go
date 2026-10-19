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

package model

// ModuleView is what Global reads about one module for its run: the CRDs under Path and the
// conversion webhooks rendered from the package object. The package Reconciler attaches it to the
// module's scheduler node (schedule.Node.Attachment) and the scheduler hands it to Global with the
// enabled set (schedule.EnabledModule), computed together: Global always reads the revision its
// event names, and keeps no copy of any package's state.
type ModuleView struct {
	Name    string // Name is the package name.
	Version string // Version is the revision's version, for messages.
	// Path is the root the object was loaded from: the attached image, or the weight-prefixed
	// directory of an embedded module.
	Path string
	// Package is the node's object; Global only renders the conversion webhooks from it and never
	// changes it.
	Package Package
}
