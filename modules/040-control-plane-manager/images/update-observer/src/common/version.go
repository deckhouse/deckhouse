package common

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func NormalizeVersion(version string) (string, error) {
	if version == "" {
		return "", nil
	}

	normalized := version
	if !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}

	// Проверяем, что версия валидна
	if !semver.IsValid(normalized) {
		return "", fmt.Errorf("invalid semver: %s", version)
	}

	parts := strings.Split(normalized, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("version must have at least MAJOR.MINOR: %s", version)
	}

	return fmt.Sprintf("%s.%s", parts[0], parts[1]), nil
}
