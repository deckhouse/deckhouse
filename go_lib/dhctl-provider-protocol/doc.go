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

// Package protocol declares nothing; it documents the module. dhctl starts a cloud
// provider's validator binary as a subprocess and calls it over gRPC on the address
// it passes in. See README.md for the wire specification.
//
// # Packages
//
//   - api/pb/<action>/v<N> — .proto sources: one service per action and version.
//
//   - api/<action>/v<N> — one package per action and wire version. It holds the
//     generated zz_generated.*.go AND the hand-written half of the contract: the
//     payload with its rules and conversions, plus the helpers for building and
//     rendering the violations the response carries.
//
//     Both halves share the package, so the zz_generated. prefix is what tells them
//     apart: `make generate` deletes and rewrites exactly those files, everything
//     else is written by hand, and the directory must never be deleted wholesale. A
//     generated message named after a hand-written type gets a distinct name in the
//     .proto — see ViolationResponse.
//
//   - server — what a validator binary runs: the listener, the panic interceptor and
//     the service bound to the validator's own implementation.
//
//   - client — what a caller uses: one method per action, no wire types exposed.
//
//   - errs — the sentinels a validator returns to ask for a specific gRPC status.
//
// # Adding an action
//
// Validation is the first action, not the only conceivable one — a binary could serve
// its own schemas, config migrations or plan rules. A new one is one new directory:
//
//  1. api/pb/<action>/v1/<action>.proto: its own service, its own messages and its
//     own proto package;
//  2. make generate;
//  3. api/<action>/v1: the payload, the result and the conversions;
//  4. server and client: the two transport halves — a Service constructor and a
//     method on the client.
//
// A binary registers the services it implements, and a caller learns a missing action
// from gRPC's Unimplemented rather than from a negotiation of its own.
//
// # Adding a wire version
//
// Because the conversions live apart from the domain types, a second wire version is
// additive: api/pb gains <action>/v2 with its own go_package (two versions in one Go
// package would collide on message names), and api/<action>/v2 the generated code
// plus its own payload, result and conversions. The server registers both versions on
// one listener, and a validator keeps implementing the same plain Go interface,
// unaware of which version a caller used.
package protocol
