package state

import (
	"fmt"
	"net"
)

func validateCIDR(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("CIDR is empty")
	}

	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return fmt.Errorf("CIDR is invalid: %v", err)
	}

	return nil
}
