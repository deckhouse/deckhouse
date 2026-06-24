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

const maxSanitizeDepth = 100

// Sanitize is a slog.ReplaceAttr function.
// It scans string values for sensitive keywords and replaces matches with [FILTERED - ...].
// Recursively processes groups. Skips control attributes (time, level, source, renderer markers).
// Idempotent - already filtered values pass through unchanged.
func Sanitize(_ []string, attr slog.Attr) slog.Attr {
	return sanitizeWithDepth(attr, 0)
}

func sanitizeWithDepth(attr slog.Attr, depth int) slog.Attr {
	if depth > maxSanitizeDepth {
		return attr
	}

	if isControlAttr(attr.Key) {
		return attr
	}

	v := attr.Value.Resolve()

	switch v.Kind() {
	case slog.KindString:
		cleaned := sanitizeMessage(v.String())
		attr.Value = slog.StringValue(cleaned)
		return attr

	case slog.KindGroup:
		group := v.Group()
		if len(group) == 0 {
			return attr
		}

		cleaned := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			cleaned = append(cleaned, sanitizeWithDepth(child, depth+1))
		}

		attr.Value = slog.GroupValue(cleaned...)
		return attr

	default:
		return attr
	}
}

// isControlAttr reports if key is a control attribute that should not be redacted.
// These include slog built-ins (time, level, source) and renderer markers
// that control terminal UI formatting.
func isControlAttr(key string) bool {
	switch key {
	case
		// built-in slog keys
		slog.TimeKey, slog.LevelKey, slog.SourceKey,
		// internal keys
		attrKeyCompact, attrKeyBadge, attrKeyBanner, attrKeyConnString,
		attrKeyProcessEvent, attrKeyProcessName,
		attrKeyProgressEvent, attrKeyProgressName, attrKeyProgressValue, attrKeyProgressTitle,
		attrKeyFileOnly:
		return true
	}
	return false
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
