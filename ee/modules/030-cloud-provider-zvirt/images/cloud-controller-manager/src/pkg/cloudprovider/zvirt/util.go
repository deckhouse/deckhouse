package zvirt

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/types"
)

var regExpProviderID = regexp.MustCompile(`^` + providerName + `://(.+)$`)

func MapNodeNameToVMName(nodeName types.NodeName) string {
	return string(nodeName)
}

func ParseProviderID(providerID string) (string, error) {
	matches := regExpProviderID.FindStringSubmatch(providerID)
	if len(matches) == 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("can't parse providerID %q", providerID)
}
