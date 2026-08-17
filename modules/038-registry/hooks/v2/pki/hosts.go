/*
Copyright 2026 Flant JSC

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

package pki

import (
	"fmt"
	"net"
	"slices"

	"github.com/deckhouse/deckhouse/go_lib/registry/pki"
)

// certificateCovers reports whether the certificate is already valid for every
// host, so an unchanged node list does not reissue anything.
func certificateCovers(certPEM string, hosts []string) (bool, error) {
	if certPEM == "" {
		return false, nil
	}

	certificate, err := pki.DecodeCertificate([]byte(certPEM))
	if err != nil {
		return false, fmt.Errorf("decoding the certificate: %w", err)
	}

	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			if !slices.ContainsFunc(certificate.IPAddresses, ip.Equal) {
				return false, nil
			}
			continue
		}
		if !slices.Contains(certificate.DNSNames, host) {
			return false, nil
		}
	}
	return true, nil
}
