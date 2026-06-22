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

package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"registry-agent/internal/config"
)

// parseRegistryConfig extracts a config.RegistryConfig from the unstructured
// representation of a RegistryConfig custom resource. Absent optional fields
// result in nil pointers / zero values, not errors. Type assertion failures in
// structurally invalid positions are returned as errors.
func parseRegistryConfig(u *unstructured.Unstructured) (config.RegistryConfig, error) {
	var cfg config.RegistryConfig

	regsRaw, found, err := unstructured.NestedSlice(u.Object, "spec", "registries")
	if err != nil {
		return config.RegistryConfig{}, fmt.Errorf("spec.registries: %w", err)
	}
	if found {
		cfg.Registries = make([]config.RegistryEntry, 0, len(regsRaw))
		for i, raw := range regsRaw {
			regMap, ok := raw.(map[string]interface{})
			if !ok {
				return config.RegistryConfig{}, fmt.Errorf("spec.registries[%d]: not a map", i)
			}
			entry, err := parseRegistryEntry(regMap)
			if err != nil {
				return config.RegistryConfig{}, fmt.Errorf("spec.registries[%d]: %w", i, err)
			}
			cfg.Registries = append(cfg.Registries, entry)
		}
	}

	usersRaw, found, err := unstructured.NestedSlice(u.Object, "spec", "auth", "users")
	if err != nil {
		return config.RegistryConfig{}, fmt.Errorf("spec.auth.users: %w", err)
	}
	if found {
		cfg.Auth.Users = make([]config.UserSpec, 0, len(usersRaw))
		for i, raw := range usersRaw {
			userMap, ok := raw.(map[string]interface{})
			if !ok {
				return config.RegistryConfig{}, fmt.Errorf("spec.auth.users[%d]: not a map", i)
			}
			name, _, _ := unstructured.NestedString(userMap, "name")
			role, _, _ := unstructured.NestedString(userMap, "role")
			cfg.Auth.Users = append(cfg.Auth.Users, config.UserSpec{Name: name, Role: role})
		}
	}

	return cfg, nil
}

// parseRegistryEntry parses a single registry entry map into a config.RegistryEntry.
func parseRegistryEntry(regMap map[string]interface{}) (config.RegistryEntry, error) {
	host, _, _ := unstructured.NestedString(regMap, "host")
	source, _, _ := unstructured.NestedString(regMap, "source")

	entry := config.RegistryEntry{
		Host:   host,
		Source: source,
	}

	// Parse optional upstream.
	upstreamMap, hasUpstream, err := unstructured.NestedMap(regMap, "upstream")
	if err != nil {
		return config.RegistryEntry{}, fmt.Errorf("upstream: %w", err)
	}
	if hasUpstream {
		upstream, err := parseUpstream(upstreamMap)
		if err != nil {
			return config.RegistryEntry{}, fmt.Errorf("upstream: %w", err)
		}
		entry.Upstream = &upstream
	}

	// Parse optional cache.
	_, hasCacheMap, err := unstructured.NestedMap(regMap, "cache")
	if err != nil {
		return config.RegistryEntry{}, fmt.Errorf("cache: %w", err)
	}
	if hasCacheMap {
		enabled, _, _ := unstructured.NestedBool(regMap, "cache", "enabled")
		entry.Cache = &config.CacheSpec{Enabled: enabled}
	}

	return entry, nil
}

// parseUpstream parses the upstream map into a config.UpstreamSpec.
func parseUpstream(upstreamMap map[string]interface{}) (config.UpstreamSpec, error) {
	host, _, _ := unstructured.NestedString(upstreamMap, "host")
	path, _, _ := unstructured.NestedString(upstreamMap, "path")
	scheme, _, _ := unstructured.NestedString(upstreamMap, "scheme")
	ca, _, _ := unstructured.NestedString(upstreamMap, "ca")

	up := config.UpstreamSpec{
		Host:   host,
		Path:   path,
		Scheme: scheme,
		CA:     ca,
	}

	// Parse optional credentials.
	credsMap, hasCreds, err := unstructured.NestedMap(upstreamMap, "credentials")
	if err != nil {
		return config.UpstreamSpec{}, fmt.Errorf("credentials: %w", err)
	}
	if hasCreds {
		username, _, _ := unstructured.NestedString(credsMap, "username")
		password, _, _ := unstructured.NestedString(credsMap, "password")
		dockerCfg, _, _ := unstructured.NestedString(credsMap, "dockerCfg")
		up.Credentials = &config.Credentials{
			Username:  username,
			Password:  password,
			DockerCfg: dockerCfg,
		}
	}

	return up, nil
}
