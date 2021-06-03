package certificate

import (
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/helpers"
)

func IsCertificateExpiringSoon(cert string, durationLeft time.Duration) (bool, error) {
	c, err := helpers.ParseCertificatePEM([]byte(cert))
	if err != nil {
		return false, fmt.Errorf("certificate cannot parsed: %v", err)
	}
	if time.Until(c.NotAfter) < durationLeft {
		return true, nil
	}
	return false, nil
}
