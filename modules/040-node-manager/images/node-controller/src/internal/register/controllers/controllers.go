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

// Package controllers triggers init()-based registration of all controllers.
// Import this package with a blank import in main.go.
package controllers

import (
	_ "github.com/deckhouse/node-controller/internal/controller/bashiblecleanup"
	_ "github.com/deckhouse/node-controller/internal/controller/draining"
	_ "github.com/deckhouse/node-controller/internal/controller/instance"
	_ "github.com/deckhouse/node-controller/internal/controller/nodegroup"
	_ "github.com/deckhouse/node-controller/internal/controller/nodetemplate"
	_ "github.com/deckhouse/node-controller/internal/controller/staticproviderid"
	_ "github.com/deckhouse/node-controller/internal/controller/updateapproval"
)
