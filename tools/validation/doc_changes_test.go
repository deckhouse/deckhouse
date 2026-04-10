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

package main

import (
	"strings"
	"testing"
)

func TestValidateModuleIncludePlaceholders(t *testing.T) {
	t.Run("valid placeholder", func(t *testing.T) {
		content := strings.Join([]string{
			`Some text.`,
			`<script type="application/x-module-include">`,
			`{"module":"operator-trivy","channel":"alpha","artifact":"feature-test1.md","onError":"fallback","fallback":"<a href=\"/modules/operator-trivy/alpha/\">Open module docs</a>"}`,
			`</script>`,
		}, "\n")

		errors := validateModuleIncludePlaceholders(content)
		if len(errors) != 0 {
			t.Fatalf("expected no errors, got %v", errors)
		}
	})

	t.Run("reject language suffix in artifact", func(t *testing.T) {
		content := `<script type="application/x-module-include">{"module":"operator-trivy","artifact":"feature-test1.ru.md"}</script>`

		errors := validateModuleIncludePlaceholders(content)
		if len(errors) != 1 || !strings.Contains(errors[0], "must not contain language suffix") {
			t.Fatalf("expected language suffix validation error, got %v", errors)
		}
	})

	t.Run("require fallback body", func(t *testing.T) {
		content := `<script type="application/x-module-include">{"module":"operator-trivy","artifact":"feature-test1.md","onError":"fallback"}</script>`

		errors := validateModuleIncludePlaceholders(content)
		if len(errors) != 1 || !strings.Contains(errors[0], "fallback content is required") {
			t.Fatalf("expected fallback validation error, got %v", errors)
		}
	})

	t.Run("reject invalid json", func(t *testing.T) {
		content := `<script type="application/x-module-include">{"module":"operator-trivy",}</script>`

		errors := validateModuleIncludePlaceholders(content)
		if len(errors) != 1 || !strings.Contains(errors[0], "invalid JSON") {
			t.Fatalf("expected invalid json error, got %v", errors)
		}
	})
}
