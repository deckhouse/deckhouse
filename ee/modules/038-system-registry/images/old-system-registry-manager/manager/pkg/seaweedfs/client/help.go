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
	DefaultMasterHttpPort = 9333
	DefaultMasterGrpcPort = 19333
	DefaultFilerHttpPort  = 8888
	DefaultFilerGrpcPort  = 18888
)

func FromIpToMasterHttpHost(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultMasterHttpPort)
}

func GenerateMasterGrpcAddressFromIP(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultMasterGrpcPort)
}

func FromIpToFilerHttpHost(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultFilerHttpPort)
}

func FromIpToFilerGrpcHost(ip string) string {
	return fmt.Sprintf("%s:%d", ip, DefaultFilerGrpcPort)
}

func GetIpFromAddress(address string) string {
	parts := strings.Split(address, ":")
	if len(parts) >= 2 {
		// If pars -> return first part
		return parts[0]
	}
	// Else -> return full address
	return address
}

func GenerateIDFromIP(ip string) string {
	return FromIpToMasterHttpHost(ip)
}
