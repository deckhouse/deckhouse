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

// Package ondemand holds the terms of the on-demand rendering of module
// documentation that its parts and adapters share: the target and the
// release channels of a module, the stages and the status of a render, the
// failures and their kinds, and the ports the adapters implement
// (RenderFunc, RenderListFunc, TagLister, ColdCache). The rendering itself is
// in the subpackages.
package ondemand
