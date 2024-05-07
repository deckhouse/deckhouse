/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pkg

import (
	"crypto/x509"
	"time"
)

func CertificateExpiresSoon(cert *x509.Certificate, duration time.Duration) bool {
	now := time.Now()

	expirationThreshold := now.Add(duration)
	return cert.NotAfter.Before(expirationThreshold)
}
