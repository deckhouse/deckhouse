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

// Package providerdata re-exports the shared protocol types from
// go_lib/dhctl-provider-protocol. All wire types and constants are defined
// there; this package provides stable import paths for dhctl internals.
package providerdata

import proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"

const (
	OperationBootstrap = proto.OperationBootstrap
	OperationConverge  = proto.OperationConverge
	OperationDestroy   = proto.OperationDestroy
)

type CloudProviderVars = proto.CloudProviderVars
type PrepareInput = proto.PrepareInput
type PrepareResult = proto.PrepareResult
type ValidateRequest = proto.ValidateRequest
type ValidateResponse = proto.ValidateResponse
type PrepareRequest = proto.PrepareRequest
type PrepareResponse = proto.PrepareResponse
