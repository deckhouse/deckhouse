/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage_class

import (
	"strings"
	"unicode"
)

// NormalizeStorageClassName turns a cloud-side name — a zVirt storage domain, a DVP storage class — into a
// name Kubernetes accepts for an object: a lowercase RFC 1123 subdomain.
//
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func NormalizeStorageClassName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		} else if r == ' ' {
			return '-'
		}

		return rune(-1)
	}

	// A lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'.
	value = strings.Map(mapFn, value)

	// It must start and end with an alphanumeric character.
	return strings.Trim(value, "-.")
}
