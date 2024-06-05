/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package client

import (
	"fmt"
	"strings"
)

const (
	DefaultMasterPort = 9333
	DefaultFilerPort  = 8888
)

func FromIpToMasterHost(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultMasterPort)
}

func FromIpToFillerHost(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultFilerPort)
}

func FromIdToIp(id string) (string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ID format")
	}
	return parts[0], nil
}

func FromIpToId(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultMasterPort)
}
