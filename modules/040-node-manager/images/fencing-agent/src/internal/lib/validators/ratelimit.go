package validators

import "fmt"

func ValidateRateLimit(rps, burst int, name string) error {
	if rps > 0 && burst == 0 {
		return fmt.Errorf("%s: burst must be > 0 when RPS > 0", name)
	}
	if rps > 0 && burst < rps {
		return fmt.Errorf("%s: burst should be >= RPS", name)
	}
	if burst > 0 && rps == 0 {
		return fmt.Errorf("%s: RPS must be > 0 when burst > 0", name)
	}
	return nil
}
