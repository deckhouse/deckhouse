package template

import (
	"fmt"
	"strings"
)

// Parses resource name that is expected to be of form {os}.{target} with hyphens as delimiters,
// e.g.
//
//	`ubuntu-lts.master`  for nodegroup bundles
//	`ubuntu-lts.1-19`    for generic bundles
func ParseName(name string) (string, string, error) {
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("name: %q must comply with format {os}.{target} using hyphens as innner delimiters", name)
	}

	os, target := parts[0], parts[1]

	return os, target, nil
}

// GetNodegroupContextKey parses context secretKey for nodegroup bundles
func GetNodegroupContextKey(name string) (string, error) {
	_, ng, err := ParseName(name)
	if err != nil {
		return "", fmt.Errorf("bad name: %v", err)
	}
	return fmt.Sprintf("bundle-%s", ng), nil
}

// GetVersionContextKey parses context secretKey for kubernetes bundles
func GetVersionContextKey(name string) (string, error) {
	_, version, err := ParseName(name)
	if err != nil {
		return "", fmt.Errorf("bad os name: %v", err)
	}
	version = strings.ReplaceAll(version, "-", ".")
	return fmt.Sprintf("bundle-%s", version), nil
}

// GetBashibleContextKey parses context secretKey bashible
func GetBashibleContextKey(name string) (string, error) {
	os, nodegroup, err := ParseName(name)
	if err != nil {
		return "", fmt.Errorf("bad bashible name: %v", err)
	}
	return fmt.Sprintf("bashible-%s-%s", os, nodegroup), nil
}
