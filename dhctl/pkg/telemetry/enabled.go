package telemetry

import "os"

// IsEnabled returns true if telemetry is enabled via the DHCTL_TRACE environment variable.
func IsEnabled() bool {
	traceValue, ok := os.LookupEnv("DHCTL_TRACE")
	return ok && traceValue != "" && traceValue != "0" && traceValue != "no"
}
