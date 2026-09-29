// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package envconfig

import (
	"os"
	"strconv"
	"time"
)

// Environment variable names the deckhouse controller reads directly, outside
// the addon-operator *Config that Load builds. They are the same contract with
// the controller Deployment manifest (modules/002-deckhouse/templates/deployment.yaml),
// so they live in this package alongside it.
//
// A few names also appear as Config fields in envconfig.go. Those are read
// twice on purpose: Load parses them into addon-operator's config, while the
// entrypoint and the pod-identity helpers below need them before that config
// exists, or from code that never sees it. Go struct tags take literals only,
// so the Config fields cannot reference these constants — when a name changes,
// both places have to change.
const (
	// EnvBundle selects the always-on module set: Default, Minimal or Managed.
	EnvBundle = "DECKHOUSE_BUNDLE"

	// EnvHA holds "true"/"false". HA is the default; "false" forces single-replica
	// mode and is honored only in dev builds.
	EnvHA = "DECKHOUSE_HA"

	// Pod identity from the downward API (EnvPod=metadata.name,
	// EnvNamespace=metadata.namespace, EnvListenAddress=status.podIP), combined
	// into the leader-election lease identity. EnvListenAddress is the operator
	// listen address, which the manifest wires to the pod IP, so it doubles as one.
	EnvPod           = "DECKHOUSE_POD"
	EnvNamespace     = "ADDON_OPERATOR_NAMESPACE"
	EnvListenAddress = "ADDON_OPERATOR_LISTEN_ADDRESS"
	EnvListenPort    = "ADDON_OPERATOR_LISTEN_PORT"
	EnvClusterDomain = "KUBERNETES_CLUSTER_DOMAIN"

	// EnvNodeName is the node the pod runs on (downward API spec.nodeName).
	EnvNodeName = "DECKHOUSE_NODE_NAME"

	// EnvSkipEntrypoint is set when the controller re-execs itself on a restart
	// signal, so the one-time entrypoint setup (chroot, symlinks) runs only once.
	EnvSkipEntrypoint = "SKIP_ENTRYPOINT_EXECUTION"

	// EnvShellChrootDir and EnvModulesDir belong to the addon-operator config
	// contract (Config.ShellChrootDir, Config.ModulesDir). They are read directly
	// in the entrypoint, to prepare the shell chroot and locate modules before the
	// operator starts.
	EnvShellChrootDir = "ADDON_OPERATOR_SHELL_CHROOT_DIR"
	EnvModulesDir     = "MODULES_DIR"

	// OTLP tracing exporter settings.
	EnvTracingOTLPEndpoint      = "TRACING_OTLP_ENDPOINT"
	EnvTracingOTLPAuthToken     = "TRACING_OTLP_AUTH_TOKEN"
	EnvTracingOTLPInsecure      = "TRACING_OTLP_INSECURE"
	EnvTracingOTLPTLSSkipVerify = "TRACING_OTLP_TLS_SKIP_VERIFY"

	// Options forwarded to the embedded dhctl CLI. EnvDhctlCLIFile backs dhctl's
	// own --file flag; the controller reads it only to tell whether the user named
	// an input source at all.
	EnvLoggerType          = "DECKHOUSE_LOGGER_TYPE"
	EnvEditor              = "DECKHOUSE_EDITOR"
	EnvKubeConfigInCluster = "DECKHOUSE_KUBE_CONFIG_IN_CLUSTER"
	EnvTmpDir              = "DECKHOUSE_TMP_DIR"
	EnvDhctlCLIFile        = "DHCTL_CLI_FILE"

	// Helm engine settings the nelm client picks up. EnvKubeContext is also
	// Config.KubeContext.
	EnvHelmDriver  = "HELM_DRIVER"
	EnvKubeContext = "KUBE_CONTEXT"

	// Endpoints the "deckhouse-controller debug" subcommands talk to.
	// EnvDebugHTTPServerAddr is also Config.DebugHTTPServerAddr;
	// EnvPackagesDebugUnixSocket is the packages-side socket and is the default of
	// the --debug-unix-socket flag.
	EnvDebugHTTPServerAddr     = "DEBUG_HTTP_SERVER_ADDR"
	EnvPackagesDebugUnixSocket = "PACKAGES_DEBUG_UNIX_SOCKET"

	// EnvIsTestsEnvironment holds "true" in test runs, where it pins values that
	// would otherwise vary per run (e.g. measured durations).
	EnvIsTestsEnvironment = "D8_IS_TESTS_ENVIRONMENT"

	// EnvPackageNelmTimeout bounds each nelm release operation on the packages
	// path (render, apply, uninstall). A Go duration string (e.g. "30m").
	EnvPackageNelmTimeout = "PACKAGE_NELM_TIMEOUT"

	// EnvDocumentationBuildTimeout caps a single upload+build round-trip from the
	// module-documentation controller to a docs-builder. Every build triggers a
	// full-site Hugo rebuild serialized across all modules, so this must exceed the
	// worst-case build time; too low a value cancels healthy builds and churns the
	// builder. A Go duration string (e.g. "120s", "2m").
	EnvDocumentationBuildTimeout = "DOCUMENTATION_BUILD_TIMEOUT"
)

// EnvOr returns the value of the env var name, or defaultValue when it is unset or empty.
func EnvOr(name, defaultValue string) string {
	if v, ok := os.LookupEnv(name); ok && v != "" {
		return v
	}
	return defaultValue
}

// EnvBoolOr parses the env var as a bool (per strconv.ParseBool), or returns
// defaultValue when unset, empty, or unparseable.
func EnvBoolOr(name string, defaultValue bool) bool {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// EnvDurationOr parses the env var as a Go duration (per time.ParseDuration), or
// returns defaultValue when unset, empty, or unparseable.
func EnvDurationOr(name string, defaultValue time.Duration) time.Duration {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// EnabledHA reports whether HA is enabled. HA is the default; it is off only when
// DECKHOUSE_HA is explicitly "false". The caller honors "off" only in dev builds.
func EnabledHA() bool { return os.Getenv(EnvHA) != "false" }

// PodName is the controller pod name, used as the leader-election identity.
func PodName() string { return os.Getenv(EnvPod) }

// PodIP is the controller pod IP. It reads the operator listen-address env,
// which the Deployment wires to status.podIP.
func PodIP() string { return os.Getenv(EnvListenAddress) }

// PodNamespace is the controller pod namespace.
func PodNamespace() string { return os.Getenv(EnvNamespace) }

// ListenAddress is the api server listen address.
func ListenAddress() string { return os.Getenv(EnvListenAddress) }

// ListenPort is the api server listen port.
func ListenPort() string { return os.Getenv(EnvListenPort) }

// ClusterDomain is the Kubernetes cluster domain.
func ClusterDomain() string { return os.Getenv(EnvClusterDomain) }

// NodeName is the name of the node the controller pod runs on.
func NodeName() string { return os.Getenv(EnvNodeName) }

// TracingOTLPEndpoint is the OTLP trace exporter endpoint.
func TracingOTLPEndpoint() string { return os.Getenv(EnvTracingOTLPEndpoint) }

// TracingOTLPAuthToken is the OTLP trace exporter auth token.
func TracingOTLPAuthToken() string { return os.Getenv(EnvTracingOTLPAuthToken) }

// TracingOTLPInsecure reports whether the OTLP exporter uses an insecure transport.
func TracingOTLPInsecure() bool { return os.Getenv(EnvTracingOTLPInsecure) == "true" }

// TracingOTLPTLSSkipVerify reports whether OTLP exporter TLS verification is skipped.
func TracingOTLPTLSSkipVerify() bool { return os.Getenv(EnvTracingOTLPTLSSkipVerify) == "true" }

// IsTestsEnvironment reports whether the process runs under the test harness.
func IsTestsEnvironment() bool { return os.Getenv(EnvIsTestsEnvironment) == "true" }

// defaultPackageNelmTimeout applies when EnvPackageNelmTimeout is unset,
// unparseable, or not positive.
const defaultPackageNelmTimeout = 30 * time.Minute

// PackageNelmTimeout is the timeout the packages path applies to one nelm
// release operation. Defaults to 30m when PACKAGE_NELM_TIMEOUT is unset,
// unparseable, or not positive.
func PackageNelmTimeout() time.Duration {
	if d := EnvDurationOr(EnvPackageNelmTimeout, defaultPackageNelmTimeout); d > 0 {
		return d
	}
	return defaultPackageNelmTimeout
}

// DocumentationBuildTimeout is the per-request timeout the module-documentation
// controller applies to docs-builder upload/build calls. Defaults to 120s when
// DOCUMENTATION_BUILD_TIMEOUT is unset or unparseable.
func DocumentationBuildTimeout() time.Duration {
	return EnvDurationOr(EnvDocumentationBuildTimeout, 120*time.Second)
}
