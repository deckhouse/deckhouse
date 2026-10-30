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

	DialTimeout      time.Duration
	FirstByteTimeout time.Duration
	KeepAlivePeriod  time.Duration
	TCPUserTimeout   time.Duration

	HealthInterval time.Duration
	HealthTimeout  time.Duration
	HealthJitter   float64
	StallTimeout   time.Duration

	FailStepInterval time.Duration
	FailStepWindow   time.Duration

	ProxyHealthListen string

	LogLevel string

	ShutdownDelay time.Duration

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

// Probe defaults. The static pod manifest does not set the probe flags, thus
// these values apply on every node.
const (
	defaultHealthInterval   = time.Second
	defaultHealthTimeout    = 500 * time.Millisecond
	defaultHealthJitter     = 0.2
	defaultFailStepInterval = 500 * time.Millisecond
	defaultFailStepWindow   = 30 * time.Second
)

// defaultShutdownDelay and the shutdown of the health server stay below the 5s
// that kubelet gives to node-critical pods on a worker shutdown. Thus the proxy
// closes the listener before kubelet kills it.
const defaultShutdownDelay = 3 * time.Second

// Parse reads command-line flags & environment variables and returns the
// resulting Config. Unknown flags will cause Parse to exit via flag.Parse.
func Parse() Config {
	var cfg Config

	listenAddr := flag.String("listen-address", getenvDefault("LISTEN_ADDRESS", "127.0.0.1"), "address to bind the load balancer to")
	listenPort := flag.Int("listen-port", getenvIntDefault("LISTEN_PORT", 6445), "Port to bind the load balancer to")

	dialTimeout := flag.Duration("dial-timeout", 5*time.Second, "Dial timeout for connections to upstreams")
	firstByteTimeout := flag.Duration("first-byte-timeout", 3*time.Second, "Floor of the per-upstream wait for the first byte on a new upstream connection before retrying another upstream; 0 disables the wait")
	keepAlivePeriod := flag.Duration("keepalive-period", 1*time.Second, "TCP keepalive period for connections")
	tcpUserTimeout := flag.Duration("tcp-user-timeout", 5*time.Second, "TCP_USER_TIMEOUT for connections (Linux only)")
	healthInterval := flag.Duration("health-interval", defaultHealthInterval, "Upstream healthcheck interval")
	healthTimeout := flag.Duration("health-timeout", defaultHealthTimeout, "Upstream healthcheck timeout")
	healthJitter := flag.Float64("health-jitter", defaultHealthJitter, "Jitter factor for healthcheck interval (0..1 recommended)")
	failStepInterval := flag.Duration("fail-step-interval", defaultFailStepInterval, "Healthcheck interval while no upstream is ready (fail-step)")
	failStepWindow := flag.Duration("fail-step-window", defaultFailStepWindow, "Longest fail-step period in one failure episode; 0 disables fail-step")
	stallTimeout := flag.Duration("stall-timeout", 3*time.Second, "Close client connections pinned to an apiserver whose /readyz gave no answer for this long while another apiserver has served 200 for twice as long; 0 disables")
	// The static pod manifest of bashible sets the same address.
	proxyHealthListen := flag.String("health-listen", getenvDefault("HEALTH_LISTEN", "127.0.0.1:6480"), "Address for the HTTP endpoints /healthz, /readyz and /upstreams (e.g., 127.0.0.1:6480)")
	logLevel := flag.String("log-level", getenvDefault("LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	shutdownDelay := flag.Duration("shutdown-delay", defaultShutdownDelay, "How long the proxy continues to serve after the first SIGTERM or SIGINT before it closes the listener; a second signal ends the delay; 0 closes the listener at once")

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
	cfg.ShutdownDelay = *shutdownDelay

	cfg.DialTimeout = *dialTimeout
	cfg.FirstByteTimeout = *firstByteTimeout
	cfg.KeepAlivePeriod = *keepAlivePeriod
	cfg.TCPUserTimeout = *tcpUserTimeout
	cfg.HealthInterval = *healthInterval
	cfg.HealthTimeout = *healthTimeout
	cfg.HealthJitter = *healthJitter
	cfg.FailStepInterval = *failStepInterval
	cfg.FailStepWindow = *failStepWindow
	cfg.StallTimeout = *stallTimeout
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
