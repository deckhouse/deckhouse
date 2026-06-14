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

package logger

import (
	"fmt"
	"log/slog"
	"strings"
)

var sensitiveKeywords = []string{
	`"name":"d8-cluster-terraform-state"`,
	`"name":"d8-provider-cluster-configuration"`,
	`"name":"d8-dhctl-converge-state"`,
	`"kind":"DexProvider"`,
	`"kind":"DexProviderList"`,
	`"kind":"ModuleConfig"`,
	`"kind":"ModuleConfigList"`,
	`"kind":"Secret"`,
	`"kind":"SecretList"`,
	`"kind":"SSHCredentials"`,
	`"kind":"SSHCredentialsList"`,
	`"kind":"ClusterLogDestination"`,
	`"kind":"ClusterLogDestinationList"`,
	`"kind":"NodeUser"`,
	`cluster-tf-state.json`,
	`cloud-provider-discovery-data.json`,
	`node-tf-state.json`,
}

// Sanitize is a slog.HandlerOptions.ReplaceAttr function: if the message attribute
// contains a sensitive keyword, the whole message is replaced with a redaction marker.
func Sanitize(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.MessageKey {
		return attr
	}
	attr.Value = slog.StringValue(sanitizeMessage(attr.Value.String()))
	return attr
}

// sanitizeMessage redacts msg to a marker when it contains a sensitive keyword, else returns it
// unchanged. It is the single redaction primitive: TerminalUIHandler.Handle applies it once before
// fan-out so both sinks see the clean message, and Sanitize wires it into JSON/text handler options.
// Idempotent — the marker carries no keyword, so a second pass is a no-op.
func sanitizeMessage(msg string) string {
	if kw := findSensitive(msg); kw != "" {
		return fmt.Sprintf("[FILTERED - %s]", kw)
	}
	return msg
}

func findSensitive(msg string) string {
	for _, kw := range sensitiveKeywords {
		if strings.Contains(msg, kw) {
			return kw
		}
	}
	return ""
}
