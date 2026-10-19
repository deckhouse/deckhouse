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

// Package model is the vocabulary of the package lifecycle: the data that flows between Engine,
// the package Reconciler, Current and Global, and the Report. It holds types, not behaviour.
//
// Dependency interfaces live with their consumers. The two interfaces here are carrier types:
// Package, the loaded package object, and Windows, the update calendar inside Policy. A value
// handed from one component to the next needs one type both can name; each consumer still calls
// it through its own narrow interface.
package model
