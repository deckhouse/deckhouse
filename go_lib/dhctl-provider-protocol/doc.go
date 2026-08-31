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
// provider's validator binary as a subprocess and calls it over gRPC on a unix
// socket. See README.md for the wire specification.
//
// # Packages
//
//   - <action> — one package per action: its payload, its result, its vocabulary,
//     the rules that hold regardless of transport, and the conversions to that
//     action's wire types. A validator imports this and nothing else of the
//     protocol. Today: validate.
//   - api/pb — .proto sources: one service per action, plus its messages.
//   - api/gen — generated from api/pb by `make proto`. Never edited by hand.
//   - server — what a validator binary runs.
//   - client — what a host uses: one method per action, no wire types exposed.
//
// # Adding an action
//
// Validation is the first action, not the only conceivable one — a binary could serve
// its own schemas, config migrations or plan rules. A new one is:
//
//  1. api/pb: its own service and messages, so a binary that does not implement it
//     simply does not pass it to server.Start and a caller sees Unimplemented;
//  2. make proto;
//  3. <action>: the payload, the result, the action's own rules, and its
//     conversions;
//  4. server and client: the two transport halves — a server.Service constructor
//     and a method on client.Client.
//
// Vocabulary shared by two actions moves up only when the second one appears.
//
// # Adding a wire version
//
// Because each action package owns its own mapping, a second wire version is
// additive: api/pb gains a <action>/v2 subdirectory with its own go_package (the
// flat api/pb holds v1 alone — two versions in one Go package would collide on
// message names), api/gen the generated code, and the action package the second
// pair of conversions. The server registers both versions on one listener, and
// validators keep implementing the same plain Go interfaces, unaware of which
// version a caller used.
package protocol
