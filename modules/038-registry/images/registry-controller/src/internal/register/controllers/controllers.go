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

// Package controllers imports every reconciler so that its init function runs.
//
// This is the single place that decides which controllers exist. main.go imports
// it blankly, so a new reconciler is one line here and cannot be forgotten
// halfway.
package controllers

import (
	_ "github.com/deckhouse/registry-controller/internal/controller/config"
	_ "github.com/deckhouse/registry-controller/internal/controller/layout"
	_ "github.com/deckhouse/registry-controller/internal/controller/observe"
)
