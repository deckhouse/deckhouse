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

package options

import (
	"time"

	otattribute "go.opentelemetry.io/otel/attribute"
)

// ServerOptions configures the dhctl gRPC server.
type ServerOptions struct {
	Network                    string
	Address                    string
	ParallelTasksLimit         int
	RequestsCounterMaxDuration time.Duration
}

func (o *ServerOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.String("server.network", o.Network),
		otattribute.String("server.address", o.Address),
		otattribute.Int("server.parallelTasksLimit", o.ParallelTasksLimit),
		otattribute.String("server.requestsCounterMaxDuration", o.RequestsCounterMaxDuration.String()),
	}
}
