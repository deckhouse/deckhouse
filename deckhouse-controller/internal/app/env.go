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

package app

import (
	"os"
	"strconv"
)

// Environment variable names the deckhouse controller reads at runtime. They are
// the contract with the controller Deployment manifest
// (modules/002-deckhouse/templates/deployment.yaml). A few overlap the
// addon-operator config and are also parsed into *Config by the envconfig
// package; they are read directly here only for early-startup and pod-identity
// needs, before that config is built.
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
	EnvClusterDomain = "KUBERNETES_CLUSTER_DOMAIN"

	// EnvNodeName is the node the pod runs on (downward API spec.nodeName).
	EnvNodeName = "DECKHOUSE_NODE_NAME"

	// EnvSkipEntrypoint is set when the controller re-execs itself on a restart
	// signal, so the one-time entrypoint setup (chroot, symlinks) runs only once.
	EnvSkipEntrypoint = "SKIP_ENTRYPOINT_EXECUTION"

	// EnvShellChrootDir and EnvModulesDir belong to the addon-operator config
	// contract (also parsed by envconfig). They are read directly in the entrypoint
	// to prepare the shell chroot and locate modules before the operator starts.
	EnvShellChrootDir = "ADDON_OPERATOR_SHELL_CHROOT_DIR"
	EnvModulesDir     = "MODULES_DIR"

	// OTLP tracing exporter settings.
	EnvTracingOTLPEndpoint      = "TRACING_OTLP_ENDPOINT"
	EnvTracingOTLPAuthToken     = "TRACING_OTLP_AUTH_TOKEN"
	EnvTracingOTLPInsecure      = "TRACING_OTLP_INSECURE"
	EnvTracingOTLPTLSSkipVerify = "TRACING_OTLP_TLS_SKIP_VERIFY"

	// Options forwarded to the embedded dhctl CLI.
	EnvLoggerType          = "DECKHOUSE_LOGGER_TYPE"
	EnvEditor              = "DECKHOUSE_EDITOR"
	EnvKubeConfigInCluster = "DECKHOUSE_KUBE_CONFIG_IN_CLUSTER"
	EnvTmpDir              = "DECKHOUSE_TMP_DIR"
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
