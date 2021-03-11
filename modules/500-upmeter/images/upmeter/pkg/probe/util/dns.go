package util

import (
	"context"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

func LookupIPsWithTimeout(domain string, timeout time.Duration) (ips []string, err error) {
	// If hostname is ip return it as is
	if IsIP(domain) {
		ips = []string{domain}
		return
	}

	resolver := net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return
	}

	ips = make([]string, 0)
	for _, addr := range addrs {
		ips = append(ips, addr.IP.String())
	}
	log.Debugf("domain '%s' resolved to %+v", domain, ips)
	return ips, nil
}

func IsIP(hostname string) bool {
	input := net.ParseIP(hostname)
	if input == nil || (input.To4() == nil && input.To16() == nil) {
		return false
	}
	return true
}
