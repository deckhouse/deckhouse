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

// Package app is deckhouse's single entry point for application configuration:
// constants, settings, CLI flags and environment variables.
//
// The package plays two roles:
//
//   - It owns deckhouse's own well-known values: product identity, the
//     d8-system and kube-system namespaces, image-layout paths, the embedded
//     and downloaded module directories, resource names, the runtime
//     environment-variable contract and feature-gate flags. These are
//     centralized here instead of being scattered across the controller.
//   - It is a thin facade over the upstream addon-operator and shell-operator
//     app packages. Deckhouse code imports this package instead of reaching
//     into flant/* directly; the facade delegates to addon-operator, which in
//     turn projects its config onto shell-operator.
package app
