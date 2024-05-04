package validate

import (
	"fmt"
	"net"
)

func CIDR(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("CIDR is empty")
	}

	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return fmt.Errorf("CIDR is invalid: %v", err)
	}

	return nil
}
