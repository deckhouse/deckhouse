// Copyright 2025 Flant JSC
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

package log

import (
	"fmt"
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
}

type LogSanitizer struct{}

func (l *LogSanitizer) Filter(args []any) []any {
	for i, arg := range args {
		str, ok := arg.(string)
		if !ok {
			continue
		}
		if matchedKeyword := findSensitiveSubstring(str); matchedKeyword != "" {
			args[i] = fmt.Sprintf(`[FILTERED - %s]`, matchedKeyword)
		}
	}
	return args
}

func (l *LogSanitizer) FilterF(format string, args []any) (string, []any) {
	return format, l.Filter(args)
}

func (l *LogSanitizer) FilterS(msg string, keysAndValues []any) (string, []any) {
	return msg, l.Filter(keysAndValues)
}

func findSensitiveSubstring(msg string) string {
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(msg, keyword) {
			return keyword
		}
	}
	return ""
}
