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

package telemetry

import "go.opentelemetry.io/otel/attribute"

// CloudSpanAttributes builds span attributes describing the cluster being
// operated on. Empty values are skipped so static clusters don't get blank
// cloud.* keys. Takes primitives (not *config.MetaConfig) to avoid a
// config->telemetry->config import cycle.
func CloudSpanAttributes(clusterType, provider, layout, prefix, uuid string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 5)
	if clusterType != "" {
		attrs = append(attrs, attribute.String("deckhouse.cluster.type", clusterType))
	}
	if provider != "" {
		attrs = append(attrs, attribute.String("deckhouse.cloud.provider", provider))
	}
	if layout != "" {
		attrs = append(attrs, attribute.String("deckhouse.cloud.layout", layout))
	}
	if prefix != "" {
		attrs = append(attrs, attribute.String("deckhouse.cluster.prefix", prefix))
	}
	if uuid != "" {
		attrs = append(attrs, attribute.String("deckhouse.cluster.uuid", uuid))
	}
	return attrs
}

// CommanderSpanAttributes builds span attributes describing how dhctl was
// invoked by Deckhouse Commander. commander_uuid is the cluster UUID in the
// Commander URL, from which a backend can build a link to the cluster page.
// Empty uuid is skipped.
func CommanderSpanAttributes(mode bool, uuid string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.Bool("dhctl.commander_mode", mode)}
	if uuid != "" {
		attrs = append(attrs, attribute.String("dhctl.commander_uuid", uuid))
	}
	return attrs
}
