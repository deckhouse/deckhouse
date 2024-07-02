package template

import (
	"fmt"
	"strings"
)

// GetNodegroupContextKey parses context secretKey for nodegroup bundles
func GetNodegroupContextKey(name string) (string, error) {
	return fmt.Sprintf("bundle-%s", name), nil
}

// GetVersionContextKey parses context secretKey for kubernetes bundles
func GetVersionContextKey(name string) (string, error) {
	version := strings.ReplaceAll(name, "-", ".")
	return fmt.Sprintf("bundle-%s", version), nil
}

// GetBashibleContextKey parses context secretKey bashible
func GetBashibleContextKey(name string) (string, error) {
	return fmt.Sprintf("bashible-%s", name), nil
}
