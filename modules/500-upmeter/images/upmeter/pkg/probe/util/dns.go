/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"context"
	"net"
	"time"
)

func LookupIPs(domain string, timeout time.Duration) (ips []string, err error) {
	// If hostname is ip return it as is
	if isIP(domain) {
		ips = []string{domain}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resolver := net.Resolver{}
	addrs, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return
	}

	for _, addr := range addrs {
		ips = append(ips, addr.IP.String())
	}
	return ips, nil
}

func isIP(hostname string) bool {
	input := net.ParseIP(hostname)
	if input == nil || (input.To4() == nil && input.To16() == nil) {
		return false
	}
	return true
}
