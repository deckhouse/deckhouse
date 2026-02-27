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

package config

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application settings parsed from flags and/or environment.
//
// All flags have environment fallbacks documented in the binary README.
type Config struct {
	ListenAddress string
	ListenPort    int

	DialTimeout     time.Duration
	KeepAlivePeriod time.Duration
	TCPUserTimeout  time.Duration

	HealthInterval time.Duration
	HealthTimeout  time.Duration
	HealthJitter   float64

	ProxyHealthListen string

	LogLevel string

	// Discovery
	DiscoverPeriod time.Duration

	// Fallback
	FallbackFile      string
	FallbackEndpoints []string

	//
	AsStaticPod bool
}

func (c Config) SLogLevel() slog.Level {
	var lvl slog.Level

	switch strings.ToLower(c.LogLevel) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return lvl
}

// Parse reads command-line flags & environment variables and returns the
// resulting Config. Unknown flags will cause Parse to exit via flag.Parse.
func Parse() Config {
	var cfg Config

	listenAddr := flag.String("listen-address", getenvDefault("LISTEN_ADDRESS", "0.0.0.0"), "address to bind the load balancer to")
	listenPort := flag.Int("listen-port", getenvIntDefault("LISTEN_PORT", 7443), "Port to bind the load balancer to")

	dialTimeout := flag.Duration("dial-timeout", 5*time.Second, "Dial timeout for connections to upstreams")
	keepAlivePeriod := flag.Duration("keepalive-period", 1*time.Second, "TCP keepalive period for connections")
	tcpUserTimeout := flag.Duration("tcp-user-timeout", 5*time.Second, "TCP_USER_TIMEOUT for connections (Linux only)")
	healthInterval := flag.Duration("health-interval", 1*time.Second, "Upstream healthcheck interval")
	healthTimeout := flag.Duration("health-timeout", 100*time.Millisecond, "Upstream healthcheck timeout")
	healthJitter := flag.Float64("health-jitter", 0.2, "Jitter factor for healthcheck interval (0..1 recommended)")
	proxyHealthListen := flag.String("health-listen", getenvDefault("HEALTH_LISTEN", ":8080"), "address for HTTP health endpoints (e.g., :8080)")
	logLevel := flag.String("log-level", getenvDefault("LOG_LEVEL", "info"), "Log level: debug|info|warn|error")

	// Discovery
	discoverPeriod := flag.Duration("discover-period", 5*time.Second, "How often to refresh kube-apiserver EndpointSlices")

	// If it starts as static pod, we must know about this
	asStaticPod := flag.Bool("as-static-pod", false, "Use kubelet certificates for getting SA with endpoint slices capabilities")

	// Fallback
	fallbackFile := flag.String("fallback-file", "", "Path to json file containing fallback upstreams (strings array)")
	fallbackUpstreams := flag.String("fallback-upstreams", "", "Comma-separated list of fallback upstreams (host:port)")

	flag.Parse()

	cfg.ListenAddress = *listenAddr
	cfg.ListenPort = *listenPort

	cfg.LogLevel = *logLevel

	cfg.DialTimeout = *dialTimeout
	cfg.KeepAlivePeriod = *keepAlivePeriod
	cfg.TCPUserTimeout = *tcpUserTimeout
	cfg.HealthInterval = *healthInterval
	cfg.HealthTimeout = *healthTimeout
	cfg.HealthJitter = *healthJitter
	cfg.ProxyHealthListen = *proxyHealthListen

	cfg.DiscoverPeriod = *discoverPeriod

	cfg.FallbackFile = *fallbackFile
	if *fallbackUpstreams != "" {
		cfg.FallbackEndpoints = strings.Split(*fallbackUpstreams, ",")
		for i, addr := range cfg.FallbackEndpoints {
			cfg.FallbackEndpoints[i] = strings.TrimSpace(addr)
		}
	}

	cfg.AsStaticPod = *asStaticPod

	return cfg
}

// getenvDefault returns the value of the environment variable or a default.
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvIntDefault returns the integer value of the environment variable or a
// default if unset or not an integer.
func getenvIntDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
