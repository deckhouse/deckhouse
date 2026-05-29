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

// Package envconfig owns the environment-variable contract between the
// deckhouse-controller deployment manifest and the addon-operator /
// shell-operator runtime libraries.
//
// addon-operator (and, transitively, shell-operator) historically read their
// configuration directly from env vars via their own ParseEnv. Starting with
// addon-operator v1.21 those env vars were namespaced under struct-tag prefixes
// (ADDON_OPERATOR, HELM, KUBE, ...): for example, the field formerly bound to
// the MODULES_DIR env var became bound to ADDON_OPERATOR_MODULES_DIR. That
// silently broke deckhouse — the long-standing MODULES_DIR set by the
// deployment was ignored, addon-operator fell back to its built-in "modules"
// default, and external modules under /deckhouse/downloaded/modules
// disappeared along with their ModuleReleases.
//
// To prevent recurrence, deckhouse-controller now parses every env var it
// cares about in one place (this package) and explicitly populates the
// addon-operator *Config. The values then propagate down the chain:
// addon-operator.ApplyConfig copies them into addon-operator's package-level
// globals (app.ModulesDir, app.GlobalHooksDir, app.KubeContext, app.TempDir,
// app.DebugKeepTmpFiles, app.LogProxyHookJSON, app.DebugHTTPServerAddr, ...),
// and bridges the one shell-operator global it still owns directly
// (shapp.DebugUnixSocket). addon-operator's own ParseEnv is intentionally not
// called by the controller so that future upstream renames cannot silently
// change the deckhouse deployment contract.
//
// Shell-operator audit (v1.16.6 kingpin -> v1.16.7+/v1.17.x cobra+envPrefix):
// shell-operator's own ParseEnv is never invoked from deckhouse-controller.
// addon-operator only registers shell-operator's Kube/ObjectPatcher/Log/Debug
// flags on its own cobra command, and v1.16.7+ kept every env name in that
// subset identical (KUBE_CONTEXT, OBJECT_PATCHER_KUBE_CLIENT_QPS, LOG_LEVEL,
// DEBUG_UNIX_SOCKET, ...). Those env vars now also live inside addon-operator's
// own *Config (Kube/ObjectPatcher/Log/Debug sub-structs) which we populate
// below, so the shell-operator refactor is invisible from this side.
//
// SHELL_OPERATOR_* App-side fallbacks (HOOKS_DIR / TMP_DIR / LISTEN_ADDRESS /
// LISTEN_PORT / PROMETHEUS_METRICS_PREFIX / NAMESPACE) are honored as a
// lower-priority alias for the corresponding addon-operator App fields. This
// mirrors addon-operator's own applyShellOperatorEnv (introduced upstream in
// commit 3d42acf "handle shell operator env"): a deployment that still ships
// shell-operator-style names keeps working, but if both the SHELL_OPERATOR_*
// and the ADDON_OPERATOR_*/historical unprefixed name are set, the
// addon-operator-side value wins. Shell-operator's standalone webhook settings
// (VALIDATING_WEBHOOK_* / CONVERSION_WEBHOOK_*) are not wired up by
// addon-operator and therefore intentionally not part of this contract —
// deckhouse uses addon-operator's own ADDON_OPERATOR_ADMISSION_* for the
// admission server.
package envconfig

import (
	"fmt"
	"os"
	"time"

	env "github.com/caarlos0/env/v11"
	ad_app "github.com/flant/addon-operator/pkg/app"
)

// Config mirrors the leaf settings of addon-operator's *Config with env tags
// that match the historical addon-operator v1.20.9 / shell-operator v1.16.6
// contract — the names that the deckhouse-controller deployment manifest
// (modules/002-deckhouse/templates/deployment.yaml) and the existing shell
// hooks have always used. addon-operator v1.21 namespaced several of these
// under ADDON_OPERATOR_; that breaking change is what motivated this package.
//
// Two env vars in particular kept their historical unprefixed names here:
//
//   - UNNUMBERED_MODULES_ORDER  (addon-operator v1.21 renamed to
//     ADDON_OPERATOR_UNNUMBERED_MODULES_ORDER)
//   - STRICT_CHECK_VALUES_MODE_ENABLED  (addon-operator v1.21 renamed to
//     ADDON_OPERATOR_STRICT_CHECK_VALUES_MODE_ENABLED)
//
// Adding a new env var to the manifest also requires adding the corresponding
// field here and a matching assignment in Apply — keeping the contract
// explicit by design.
type Config struct {
	// App settings.
	ModulesDir              string `env:"MODULES_DIR"`
	GlobalHooksDir          string `env:"GLOBAL_HOOKS_DIR"`
	TempDir                 string `env:"ADDON_OPERATOR_TMP_DIR"`
	Namespace               string `env:"ADDON_OPERATOR_NAMESPACE"`
	ListenAddress           string `env:"ADDON_OPERATOR_LISTEN_ADDRESS"`
	ListenPort              string `env:"ADDON_OPERATOR_LISTEN_PORT"`
	ConfigMapName           string `env:"ADDON_OPERATOR_CONFIG_MAP"`
	PrometheusMetricsPrefix string `env:"ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX"`
	UnnumberedModuleOrder   int    `env:"UNNUMBERED_MODULES_ORDER"`
	ShellChrootDir          string `env:"ADDON_OPERATOR_SHELL_CHROOT_DIR"`
	StrictModeEnabled       bool   `env:"STRICT_CHECK_VALUES_MODE_ENABLED"`
	AppliedExtenders        string `env:"ADDON_OPERATOR_APPLIED_MODULE_EXTENDERS"`
	ExtraLabels             string `env:"ADDON_OPERATOR_CRD_EXTRA_LABELS"`
	CRDsFilters             string `env:"ADDON_OPERATOR_CRD_FILTER_PREFIXES"`

	// Helm settings.
	HelmHistoryMax             int32         `env:"HELM_HISTORY_MAX"`
	HelmTimeout                time.Duration `env:"HELM_TIMEOUT"`
	HelmIgnoreRelease          string        `env:"HELM_IGNORE_RELEASE"`
	HelmMonitorKubeClientQPS   float32       `env:"HELM_MONITOR_KUBE_CLIENT_QPS"`
	HelmMonitorKubeClientBurst int           `env:"HELM_MONITOR_KUBE_CLIENT_BURST"`

	// Admission settings.
	AdmissionServerListenPort string `env:"ADDON_OPERATOR_ADMISSION_SERVER_LISTEN_PORT"`
	AdmissionServerCertsDir   string `env:"ADDON_OPERATOR_ADMISSION_SERVER_CERTS_DIR"`
	AdmissionServerEnabled    bool   `env:"ADDON_OPERATOR_ADMISSION_SERVER_ENABLED"`

	// Kube settings.
	KubeContext     string  `env:"KUBE_CONTEXT"`
	KubeConfig      string  `env:"KUBE_CONFIG"`
	KubeServer      string  `env:"KUBE_SERVER"`
	KubeClientQPS   float32 `env:"KUBE_CLIENT_QPS"`
	KubeClientBurst int     `env:"KUBE_CLIENT_BURST"`

	// Object patcher settings (separate Kubernetes client).
	ObjectPatcherKubeClientQPS     float32       `env:"OBJECT_PATCHER_KUBE_CLIENT_QPS"`
	ObjectPatcherKubeClientBurst   int           `env:"OBJECT_PATCHER_KUBE_CLIENT_BURST"`
	ObjectPatcherKubeClientTimeout time.Duration `env:"OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT"`

	// Debug settings (shared between addon-operator and shell-operator
	// — addon-operator copies DebugUnixSocket into the shapp global).
	DebugUnixSocket     string `env:"DEBUG_UNIX_SOCKET"`
	DebugHTTPServerAddr string `env:"DEBUG_HTTP_SERVER_ADDR"`
	DebugKeepTmpFiles   bool   `env:"DEBUG_KEEP_TMP_FILES"`
	DebugKubernetesAPI  bool   `env:"DEBUG_KUBERNETES_API"`

	// Log settings (shared between addon-operator and shell-operator).
	LogLevel         string `env:"LOG_LEVEL"`
	LogType          string `env:"LOG_TYPE"`
	LogNoTime        bool   `env:"LOG_NO_TIME"`
	LogProxyHookJSON bool   `env:"LOG_PROXY_HOOK_JSON"`
}

// Load is the single entry point for env-driven configuration of
// addon-operator (and the shell-operator globals addon-operator manages).
//
// It seeds a Config from cfg so that addon-operator's hardcoded defaults from
// ad_app.NewConfig stay in place when an env var is absent, then applies the
// SHELL_OPERATOR_* fallbacks (lower priority), then overlays the
// ADDON_OPERATOR_*/historical unprefixed env vars (higher priority — they
// always win on conflict), then copies everything back into cfg. Callers
// should follow this with ad_app.BindFlags so CLI flags still win over env
// values.
func Load(cfg *ad_app.Config) error {
	c := fromAddonOperator(cfg)
	applyShellOperatorEnv(c)
	if err := env.ParseWithOptions(c, env.Options{}); err != nil {
		return fmt.Errorf("parse deckhouse env config: %w", err)
	}
	c.Apply(cfg)
	return nil
}

// applyShellOperatorEnv copies the six SHELL_OPERATOR_* App-side env vars
// into the corresponding addon-operator fields, replicating the upstream
// addon-operator helper of the same name (commit 3d42acf "handle shell
// operator env"). These names predate addon-operator's namespacing and may
// still appear in older Deckhouse-derived deployments; honoring them keeps
// such deployments working without rewriting their manifests.
//
// Precedence is achieved by call order in Load: this runs first so that the
// ADDON_OPERATOR_* / historical unprefixed overlay applied by
// env.ParseWithOptions afterwards always wins on conflict. The mapping is
// intentionally an explicit list (no struct tags, no reflection) so a
// reviewer can see at a glance exactly which shell-operator envs the
// deckhouse-controller honors.
func applyShellOperatorEnv(c *Config) {
	if v, ok := os.LookupEnv("SHELL_OPERATOR_HOOKS_DIR"); ok {
		c.GlobalHooksDir = v
	}
	if v, ok := os.LookupEnv("SHELL_OPERATOR_TMP_DIR"); ok {
		c.TempDir = v
	}
	if v, ok := os.LookupEnv("SHELL_OPERATOR_LISTEN_ADDRESS"); ok {
		c.ListenAddress = v
	}
	if v, ok := os.LookupEnv("SHELL_OPERATOR_LISTEN_PORT"); ok {
		c.ListenPort = v
	}
	if v, ok := os.LookupEnv("SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX"); ok {
		c.PrometheusMetricsPrefix = v
	}
	if v, ok := os.LookupEnv("SHELL_OPERATOR_NAMESPACE"); ok {
		c.Namespace = v
	}
}

// Apply copies parsed env values into cfg, overwriting whatever was there.
// Exposed for tests and for callers that want to drive a *Config directly
// (e.g. when seeding addon-operator from a non-env source).
func (c *Config) Apply(cfg *ad_app.Config) {
	cfg.App.ModulesDir = c.ModulesDir
	cfg.App.GlobalHooksDir = c.GlobalHooksDir
	cfg.App.TempDir = c.TempDir
	cfg.App.Namespace = c.Namespace
	cfg.App.ListenAddress = c.ListenAddress
	cfg.App.ListenPort = c.ListenPort
	cfg.App.ConfigMapName = c.ConfigMapName
	cfg.App.PrometheusMetricsPrefix = c.PrometheusMetricsPrefix
	cfg.App.UnnumberedModuleOrder = c.UnnumberedModuleOrder
	cfg.App.ShellChrootDir = c.ShellChrootDir
	cfg.App.StrictModeEnabled = c.StrictModeEnabled
	cfg.App.AppliedExtenders = c.AppliedExtenders
	cfg.App.ExtraLabels = c.ExtraLabels
	cfg.App.CRDsFilters = c.CRDsFilters

	cfg.Helm.HistoryMax = c.HelmHistoryMax
	cfg.Helm.Timeout = c.HelmTimeout
	cfg.Helm.IgnoreRelease = c.HelmIgnoreRelease
	cfg.Helm.MonitorKubeClientQps = c.HelmMonitorKubeClientQPS
	cfg.Helm.MonitorKubeClientBurst = c.HelmMonitorKubeClientBurst

	cfg.Admission.ListenPort = c.AdmissionServerListenPort
	cfg.Admission.CertsDir = c.AdmissionServerCertsDir
	cfg.Admission.Enabled = c.AdmissionServerEnabled

	cfg.Kube.Context = c.KubeContext
	cfg.Kube.Config = c.KubeConfig
	cfg.Kube.Server = c.KubeServer
	cfg.Kube.ClientQPS = c.KubeClientQPS
	cfg.Kube.ClientBurst = c.KubeClientBurst

	cfg.ObjectPatcher.KubeClientQPS = c.ObjectPatcherKubeClientQPS
	cfg.ObjectPatcher.KubeClientBurst = c.ObjectPatcherKubeClientBurst
	cfg.ObjectPatcher.KubeClientTimeout = c.ObjectPatcherKubeClientTimeout

	cfg.Debug.UnixSocket = c.DebugUnixSocket
	cfg.Debug.HTTPServerAddr = c.DebugHTTPServerAddr
	cfg.Debug.KeepTmpFiles = c.DebugKeepTmpFiles
	cfg.Debug.KubernetesAPI = c.DebugKubernetesAPI

	cfg.Log.Level = c.LogLevel
	cfg.Log.Type = c.LogType
	cfg.Log.NoTime = c.LogNoTime
	cfg.Log.ProxyHookJSON = c.LogProxyHookJSON
}

// fromAddonOperator snapshots cfg into a Config so that env parsing can use
// addon-operator's hardcoded defaults as the baseline.
func fromAddonOperator(cfg *ad_app.Config) *Config {
	return &Config{
		ModulesDir:                     cfg.App.ModulesDir,
		GlobalHooksDir:                 cfg.App.GlobalHooksDir,
		TempDir:                        cfg.App.TempDir,
		Namespace:                      cfg.App.Namespace,
		ListenAddress:                  cfg.App.ListenAddress,
		ListenPort:                     cfg.App.ListenPort,
		ConfigMapName:                  cfg.App.ConfigMapName,
		PrometheusMetricsPrefix:        cfg.App.PrometheusMetricsPrefix,
		UnnumberedModuleOrder:          cfg.App.UnnumberedModuleOrder,
		ShellChrootDir:                 cfg.App.ShellChrootDir,
		StrictModeEnabled:              cfg.App.StrictModeEnabled,
		AppliedExtenders:               cfg.App.AppliedExtenders,
		ExtraLabels:                    cfg.App.ExtraLabels,
		CRDsFilters:                    cfg.App.CRDsFilters,
		HelmHistoryMax:                 cfg.Helm.HistoryMax,
		HelmTimeout:                    cfg.Helm.Timeout,
		HelmIgnoreRelease:              cfg.Helm.IgnoreRelease,
		HelmMonitorKubeClientQPS:       cfg.Helm.MonitorKubeClientQps,
		HelmMonitorKubeClientBurst:     cfg.Helm.MonitorKubeClientBurst,
		AdmissionServerListenPort:      cfg.Admission.ListenPort,
		AdmissionServerCertsDir:        cfg.Admission.CertsDir,
		AdmissionServerEnabled:         cfg.Admission.Enabled,
		KubeContext:                    cfg.Kube.Context,
		KubeConfig:                     cfg.Kube.Config,
		KubeServer:                     cfg.Kube.Server,
		KubeClientQPS:                  cfg.Kube.ClientQPS,
		KubeClientBurst:                cfg.Kube.ClientBurst,
		ObjectPatcherKubeClientQPS:     cfg.ObjectPatcher.KubeClientQPS,
		ObjectPatcherKubeClientBurst:   cfg.ObjectPatcher.KubeClientBurst,
		ObjectPatcherKubeClientTimeout: cfg.ObjectPatcher.KubeClientTimeout,
		DebugUnixSocket:                cfg.Debug.UnixSocket,
		DebugHTTPServerAddr:            cfg.Debug.HTTPServerAddr,
		DebugKeepTmpFiles:              cfg.Debug.KeepTmpFiles,
		DebugKubernetesAPI:             cfg.Debug.KubernetesAPI,
		LogLevel:                       cfg.Log.Level,
		LogType:                        cfg.Log.Type,
		LogNoTime:                      cfg.Log.NoTime,
		LogProxyHookJSON:               cfg.Log.ProxyHookJSON,
	}
}
