/*
Copyright 2024 Flant JSC

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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

type OSSItem struct {
	Name        string `yaml:"name"`
	Link        string `yaml:"link,omitempty"`
	Description string `yaml:"description,omitempty"`
	Logo        string `yaml:"logo,omitempty"`
	License     string `yaml:"license,omitempty"`
	ID          string `yaml:"id,omitempty"`
	Version     string `yaml:"version,omitempty"`
}

type OSSData map[string][]OSSItem

func main() {
	var (
		sourceDir  = flag.String("source", ".", "Source directory to search for oss.yaml files")
		outputFile = flag.String("output", "", "Output YAML file path (required)")
	)
	flag.Parse()

	if *outputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: output file is required\n")
		os.Exit(1)
	}

	ossData := make(OSSData)

	// Find all oss.yaml files
	err := filepath.Walk(*sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not oss.yaml
		if info.Name() != "oss.yaml" {
			return nil
		}

		// Read and parse oss.yaml
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var items []OSSItem
		if err := yaml.Unmarshal(data, &items); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Extract module name from path
		// Path format: modules/XXX-module-name/oss.yaml or ee/modules/XXX-module-name/oss.yaml
		moduleName := extractModuleName(path)
		if moduleName == "" {
			// Skip if we can't extract module name
			return nil
		}

		// Merge items for the same module
		if existing, ok := ossData[moduleName]; ok {
			ossData[moduleName] = append(existing, items...)
		} else {
			ossData[moduleName] = items
		}

		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Sort module names for consistent output
	moduleNames := make([]string, 0, len(ossData))
	for name := range ossData {
		moduleNames = append(moduleNames, name)
	}
	sort.Strings(moduleNames)

	// Create sorted output
	sortedData := make(OSSData)
	for _, name := range moduleNames {
		sortedData[name] = ossData[name]
	}

	// Write output
	output, err := yaml.Marshal(sortedData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal YAML: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outputFile, output, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated OSS data for %d modules\n", len(sortedData))
}

// extractModuleName extracts module name from path, removing numeric prefix
// Examples:
//   - modules/101-cert-manager/oss.yaml -> cert-manager
//   - ee/modules/450-keepalived/oss.yaml -> keepalived
//   - modules/000-common/oss.yaml -> common
func extractModuleName(path string) string {
	// Normalize path separators
	path = filepath.ToSlash(path)

	// Pattern to match: .../modules/XXX-module-name/oss.yaml or .../XXX-module-name/oss.yaml
	// Also handle ee/modules, ee/se/modules, etc.
	patterns := []*regexp.Regexp{
		// modules/XXX-module-name/oss.yaml
		regexp.MustCompile(`modules/(\d{3})-(.+)/oss\.yaml$`),
		// ee/modules/XXX-module-name/oss.yaml
		regexp.MustCompile(`ee/(?:se/|se-plus/|be/|fe/)?modules/(\d{3})-(.+)/oss\.yaml$`),
		// Direct module path without modules/ prefix (less common)
		regexp.MustCompile(`/(\d{3})-(.+)/oss\.yaml$`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(path)
		if len(matches) >= 3 {
			// Return the module name (second capture group)
			return matches[2]
		}
	}

	// Fallback: try to extract from any path ending with /oss.yaml
	// Remove oss.yaml and get the last directory name
	dir := filepath.Dir(path)
	base := filepath.Base(dir)

	// Try to remove numeric prefix if present
	if match := regexp.MustCompile(`^(\d{3})-(.+)$`).FindStringSubmatch(base); len(match) >= 3 {
		return match[2]
	}

	// If no numeric prefix, return as is
	return base
}
