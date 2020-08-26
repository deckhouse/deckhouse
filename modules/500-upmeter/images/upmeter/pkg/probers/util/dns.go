package util

import (
	"context"
	"fmt"
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
	log.Infof("domain '%s' resolved to %+v", domain, ips)
	return ips, nil
}

func LookupIPs(domain string) (ips []string, err error) {
	ips = make([]string, 0)

	// If hostname is ip return it as is
	if IsIP(domain) {
		ips = append(ips, domain)
		return
	}

	lookupIps, err := net.LookupIP(domain)
	if err != nil {
		return
	}
	if len(lookupIps) == 0 {
		err = fmt.Errorf("Host '%s' has no ip addresses", domain)
		return
	}

	for _, ip := range lookupIps {
		if ip.To4() != nil {
			ips = append(ips, ip.To4().String())
		}
	}

	return
}

func IsIP(hostname string) bool {
	input := net.ParseIP(hostname)
	if input == nil || (input.To4() == nil && input.To16() == nil) {
		return false
	}
	return true
}
