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

// Package protocol documents the module; it declares nothing. Everything a caller
// needs lives in the packages below, so an import always says which part of the
// protocol it depends on.
//
// A host starts a cloud provider plugin as a subprocess and calls it over gRPC on a
// unix socket. See README.md for the wire specification.
//
// # Packages
//
//   - <action> — one package per action: its payload, its result, the vocabulary it
//     needs, the function a plugin implements, the rules that hold regardless of
//     transport, and the translation to and from that action's wire types. A plugin
//     imports this and nothing else. Today: validate.
//   - api/pb/<action>/v1 — .proto sources, one service per action.
//   - api/gen/<action>/v1 — code generated from api/pb by `make proto`. Never edited
//     by hand.
//   - server — what a plugin binary runs: the gRPC service bound to the plugin's
//     implementation.
//   - client — what a host uses: one method per action, no wire types exposed.
//
// # Adding an action
//
// Validation is the first action, not the only conceivable one — a plugin could serve
// its own schemas, config migrations or plan rules. A new one is:
//
//  1. api/pb/<action>/v1/: its own service, so a plugin that does not implement it
//     simply does not register it and a caller sees Unimplemented;
//  2. make proto;
//  3. <action>: the payload, the result, the plugin function type, the action's own
//     rules, and its conversions;
//  4. server and client: the two transport halves.
//
// Vocabulary shared by two actions moves up only when the second one appears: until
// then it belongs to the action that uses it.
//
// # Adding a wire version
//
// Because each action package owns its own mapping, a second wire version is
// additive: api/pb gains <action>/v2, api/gen the generated code, and the action
// package the second pair of conversions. The server can register both versions on
// one listener, and plugins keep implementing the same plain Go functions, unaware of
// which version a caller used.
package protocol
