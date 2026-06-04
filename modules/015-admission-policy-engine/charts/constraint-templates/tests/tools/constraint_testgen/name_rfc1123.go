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
	"fmt"
	"regexp"
	"strings"
)

var rfc1123SubdomainRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

func validateRFC1123SubdomainName(raw string) error {
	if raw == "" {
		return fmt.Errorf("name is empty")
	}
	if !rfc1123SubdomainRe.MatchString(raw) {
		return fmt.Errorf("name %q does not match RFC1123 subdomain", raw)
	}
	return nil
}

func normalizeRFC1123SubdomainName(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", fmt.Errorf("name is empty")
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case ('a' <= r && r <= 'z') || ('0' <= r && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	candidate := b.String()
	for strings.Contains(candidate, "--") {
		candidate = strings.ReplaceAll(candidate, "--", "-")
	}
	for strings.Contains(candidate, "..") {
		candidate = strings.ReplaceAll(candidate, "..", ".")
	}

	parts := strings.Split(candidate, ".")
	normalizedParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "-")
		for strings.Contains(p, "--") {
			p = strings.ReplaceAll(p, "--", "-")
		}
		if p == "" {
			continue
		}
		normalizedParts = append(normalizedParts, p)
	}

	normalized := strings.Join(normalizedParts, ".")
	if normalized == "" {
		return "", fmt.Errorf("normalized name is empty")
	}
	if !rfc1123SubdomainRe.MatchString(normalized) {
		return "", fmt.Errorf("normalized name %q does not match RFC1123 subdomain", normalized)
	}
	return normalized, nil
}

func ensureObjectMetadataNameRFC1123StrictOrFallback(doc any, fallbackName, context string) error {
	m, ok := doc.(map[string]interface{})
	if !ok {
		return nil
	}

	meta, _ := m["metadata"].(map[string]interface{})
	if meta == nil {
		meta = map[string]interface{}{}
		m["metadata"] = meta
	}

	rawName, _ := meta["name"].(string)
	if rawName != "" {
		if err := validateRFC1123SubdomainName(rawName); err != nil {
			if strings.TrimSpace(context) == "" {
				return fmt.Errorf("metadata.name: %w", err)
			}
			return fmt.Errorf("%s metadata.name: %w", context, err)
		}
		return nil
	}

	normalized, err := normalizeRFC1123SubdomainName(fallbackName)
	if err != nil {
		if strings.TrimSpace(context) == "" {
			return fmt.Errorf("metadata.name fallback: %w", err)
		}
		return fmt.Errorf("%s metadata.name fallback: %w", context, err)
	}
	meta["name"] = normalized
	return nil
}

func validatePodExceptionLabelRFC1123(doc any, context string) error {
	m, ok := doc.(map[string]interface{})
	if !ok {
		return nil
	}
	meta, _ := m["metadata"].(map[string]interface{})
	if meta == nil {
		return nil
	}
	labels, _ := meta["labels"].(map[string]interface{})
	if labels == nil {
		return nil
	}
	v, exists := labels[spePodLabelKey]
	if !exists {
		return nil
	}
	labelValue, _ := v.(string)
	if err := validateRFC1123SubdomainName(labelValue); err != nil {
		if strings.TrimSpace(context) == "" {
			return fmt.Errorf("metadata.labels.%s: %w", spePodLabelKey, err)
		}
		return fmt.Errorf("%s metadata.labels.%s: %w", context, spePodLabelKey, err)
	}
	return nil
}
