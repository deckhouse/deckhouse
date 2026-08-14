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

package providercheck

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
)

const unreachableHint = " Dex login through this provider will fail the same way. If the cluster has no network path to that host (closed contour), this is expected — use an internal IdP or local users."

func isNetworkUnreachable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isNetworkUnreachable(urlErr.Err)
	}
	lower := strings.ToLower(err.Error())
	for _, needle := range []string{
		"timeout",
		"no such host",
		"connection refused",
		"network is unreachable",
		"no route to host",
		"i/o timeout",
		"tls handshake timeout",
		"connection reset",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
