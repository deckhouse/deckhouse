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
	"testing"
	"time"

	ad_app "github.com/flant/addon-operator/pkg/app"
	sh_debug "github.com/flant/shell-operator/pkg/debug"
)

// TestLoad_RegressionMODULES_DIR pins the original bug fix: the deployment
// manifest sets MODULES_DIR (no ADDON_OPERATOR_ prefix), and Load must
// surface it into cfg.App.ModulesDir, overriding addon-operator's "modules"
// default. Without this, deckhouse cannot see /deckhouse/downloaded/modules
// and garbage-collects external module releases.
func TestLoad_RegressionMODULES_DIR(t *testing.T) {
	t.Setenv("MODULES_DIR", "/deckhouse/modules:/deckhouse/downloaded/modules")

	cfg := ad_app.NewConfig()
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	const want = "/deckhouse/modules:/deckhouse/downloaded/modules"
	if cfg.App.ModulesDir != want {
		t.Fatalf("ModulesDir: got %q, want %q", cfg.App.ModulesDir, want)
	}
}

// TestLoad_GLOBAL_HOOKS_DIR mirrors the MODULES_DIR fix for the sibling
// legacy unprefixed env var (used by deckhouse werf configs and shell hooks).
func TestLoad_GLOBAL_HOOKS_DIR(t *testing.T) {
	t.Setenv("GLOBAL_HOOKS_DIR", "/deckhouse/global-hooks")

	cfg := ad_app.NewConfig()
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := cfg.App.GlobalHooksDir; got != "/deckhouse/global-hooks" {
		t.Fatalf("GlobalHooksDir: got %q, want %q", got, "/deckhouse/global-hooks")
	}
}

// TestLoad_KeepsAddonOperatorDefaults verifies that fields whose env var is
// not set keep the value seeded from ad_app.NewConfig. This is what allows us
// to bypass ad_app.ParseEnv without losing addon-operator's built-in defaults.
func TestLoad_KeepsAddonOperatorDefaults(t *testing.T) {
	cfg := ad_app.NewConfig()
	defaults := *cfg

	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.App.ListenAddress != defaults.App.ListenAddress {
		t.Errorf("ListenAddress: got %q, want default %q",
			cfg.App.ListenAddress, defaults.App.ListenAddress)
	}
	if cfg.App.ListenPort != defaults.App.ListenPort {
		t.Errorf("ListenPort: got %q, want default %q",
			cfg.App.ListenPort, defaults.App.ListenPort)
	}
	if cfg.App.ConfigMapName != defaults.App.ConfigMapName {
		t.Errorf("ConfigMapName: got %q, want default %q",
			cfg.App.ConfigMapName, defaults.App.ConfigMapName)
	}
	if cfg.App.PrometheusMetricsPrefix != defaults.App.PrometheusMetricsPrefix {
		t.Errorf("PrometheusMetricsPrefix: got %q, want default %q",
			cfg.App.PrometheusMetricsPrefix, defaults.App.PrometheusMetricsPrefix)
	}
	if cfg.Helm.HistoryMax != defaults.Helm.HistoryMax {
		t.Errorf("Helm.HistoryMax: got %d, want default %d",
			cfg.Helm.HistoryMax, defaults.Helm.HistoryMax)
	}
	if cfg.Helm.Timeout != defaults.Helm.Timeout {
		t.Errorf("Helm.Timeout: got %s, want default %s",
			cfg.Helm.Timeout, defaults.Helm.Timeout)
	}
	if cfg.Kube.ClientQPS != defaults.Kube.ClientQPS {
		t.Errorf("Kube.ClientQPS: got %v, want default %v",
			cfg.Kube.ClientQPS, defaults.Kube.ClientQPS)
	}
}

// TestLoad_AllFields covers the full env -> cfg mapping using the exact env
// var names that appear in the deckhouse-controller deployment manifest. This
// is intentionally exhaustive: if a future contributor renames an env var in
// the manifest without updating envconfig, this test will fail loudly.
func TestLoad_AllFields(t *testing.T) {
	envs := map[string]string{
		// App.
		"MODULES_DIR":                              "/d/mod:/d/dl/mod",
		"GLOBAL_HOOKS_DIR":                         "/d/gh",
		"ADDON_OPERATOR_TMP_DIR":                   "/tmp/d8",
		"ADDON_OPERATOR_NAMESPACE":                 "d8-system",
		"ADDON_OPERATOR_LISTEN_ADDRESS":            "127.0.0.1",
		"ADDON_OPERATOR_LISTEN_PORT":               "4222",
		"ADDON_OPERATOR_CONFIG_MAP":                "deckhouse",
		"ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX": "deckhouse_",
		// Historically (addon-operator v1.20.9 and earlier) these two env vars
		// were registered WITHOUT the ADDON_OPERATOR_ prefix. We pin the
		// historical contract — see envconfig.Config.
		"UNNUMBERED_MODULES_ORDER":                "42",
		"ADDON_OPERATOR_SHELL_CHROOT_DIR":         "/chroot",
		"STRICT_CHECK_VALUES_MODE_ENABLED":        "true",
		"ADDON_OPERATOR_APPLIED_MODULE_EXTENDERS": "Static,KubeConfig",
		"ADDON_OPERATOR_CRD_EXTRA_LABELS":         "heritage=deckhouse",
		"ADDON_OPERATOR_CRD_FILTER_PREFIXES":      "doc-,_",

		// Helm.
		"HELM_HISTORY_MAX":               "3",
		"HELM_TIMEOUT":                   "15m",
		"HELM_IGNORE_RELEASE":            "deckhouse",
		"HELM_MONITOR_KUBE_CLIENT_QPS":   "15",
		"HELM_MONITOR_KUBE_CLIENT_BURST": "30",

		// Admission.
		"ADDON_OPERATOR_ADMISSION_SERVER_LISTEN_PORT": "4223",
		"ADDON_OPERATOR_ADMISSION_SERVER_CERTS_DIR":   "/certs",
		"ADDON_OPERATOR_ADMISSION_SERVER_ENABLED":     "true",

		// Kube.
		"KUBE_CONTEXT":     "ctx",
		"KUBE_CONFIG":      "/k/cfg",
		"KUBE_SERVER":      "https://kube",
		"KUBE_CLIENT_QPS":  "20",
		"KUBE_CLIENT_BURST": "40",

		// ObjectPatcher.
		"OBJECT_PATCHER_KUBE_CLIENT_QPS":     "30",
		"OBJECT_PATCHER_KUBE_CLIENT_BURST":   "60",
		"OBJECT_PATCHER_KUBE_CLIENT_TIMEOUT": "15s",

		// Debug.
		"DEBUG_UNIX_SOCKET":      "/tmp/shell-operator-debug.socket",
		"DEBUG_HTTP_SERVER_ADDR": "127.0.0.1:9652",
		"DEBUG_KEEP_TMP_FILES":   "true",
		"DEBUG_KUBERNETES_API":   "true",

		// Log.
		"LOG_LEVEL":           "debug",
		"LOG_TYPE":            "json",
		"LOG_NO_TIME":         "true",
		"LOG_PROXY_HOOK_JSON": "true",
	}
	for k, v := range envs {
		t.Setenv(k, v)
	}

	cfg := ad_app.NewConfig()
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"App.ModulesDir", cfg.App.ModulesDir, "/d/mod:/d/dl/mod"},
		{"App.GlobalHooksDir", cfg.App.GlobalHooksDir, "/d/gh"},
		{"App.TempDir", cfg.App.TempDir, "/tmp/d8"},
		{"App.Namespace", cfg.App.Namespace, "d8-system"},
		{"App.ListenAddress", cfg.App.ListenAddress, "127.0.0.1"},
		{"App.ListenPort", cfg.App.ListenPort, "4222"},
		{"App.ConfigMapName", cfg.App.ConfigMapName, "deckhouse"},
		{"App.PrometheusMetricsPrefix", cfg.App.PrometheusMetricsPrefix, "deckhouse_"},
		{"App.UnnumberedModuleOrder", cfg.App.UnnumberedModuleOrder, 42},
		{"App.ShellChrootDir", cfg.App.ShellChrootDir, "/chroot"},
		{"App.StrictModeEnabled", cfg.App.StrictModeEnabled, true},
		{"App.AppliedExtenders", cfg.App.AppliedExtenders, "Static,KubeConfig"},
		{"App.ExtraLabels", cfg.App.ExtraLabels, "heritage=deckhouse"},
		{"App.CRDsFilters", cfg.App.CRDsFilters, "doc-,_"},

		{"Helm.HistoryMax", cfg.Helm.HistoryMax, int32(3)},
		{"Helm.Timeout", cfg.Helm.Timeout, 15 * time.Minute},
		{"Helm.IgnoreRelease", cfg.Helm.IgnoreRelease, "deckhouse"},
		{"Helm.MonitorKubeClientQps", cfg.Helm.MonitorKubeClientQps, float32(15)},
		{"Helm.MonitorKubeClientBurst", cfg.Helm.MonitorKubeClientBurst, 30},

		{"Admission.ListenPort", cfg.Admission.ListenPort, "4223"},
		{"Admission.CertsDir", cfg.Admission.CertsDir, "/certs"},
		{"Admission.Enabled", cfg.Admission.Enabled, true},

		{"Kube.Context", cfg.Kube.Context, "ctx"},
		{"Kube.Config", cfg.Kube.Config, "/k/cfg"},
		{"Kube.Server", cfg.Kube.Server, "https://kube"},
		{"Kube.ClientQPS", cfg.Kube.ClientQPS, float32(20)},
		{"Kube.ClientBurst", cfg.Kube.ClientBurst, 40},

		{"ObjectPatcher.KubeClientQPS", cfg.ObjectPatcher.KubeClientQPS, float32(30)},
		{"ObjectPatcher.KubeClientBurst", cfg.ObjectPatcher.KubeClientBurst, 60},
		{"ObjectPatcher.KubeClientTimeout", cfg.ObjectPatcher.KubeClientTimeout, 15 * time.Second},

		{"Debug.UnixSocket", cfg.Debug.UnixSocket, "/tmp/shell-operator-debug.socket"},
		{"Debug.HTTPServerAddr", cfg.Debug.HTTPServerAddr, "127.0.0.1:9652"},
		{"Debug.KeepTmpFiles", cfg.Debug.KeepTmpFiles, true},
		{"Debug.KubernetesAPI", cfg.Debug.KubernetesAPI, true},

		{"Log.Level", cfg.Log.Level, "debug"},
		{"Log.Type", cfg.Log.Type, "json"},
		{"Log.NoTime", cfg.Log.NoTime, true},
		{"Log.ProxyHookJSON", cfg.Log.ProxyHookJSON, true},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v (%T), want %v (%T)", c.name, c.got, c.got, c.want, c.want)
		}
	}
}

// TestLoad_ShellOperatorEnvFallback pins the SHELL_OPERATOR_* App-side
// fallbacks introduced upstream in addon-operator commit 3d42acf
// "handle shell operator env". A deployment that still uses the
// shell-operator-style names (no ADDON_OPERATOR_ prefix, no
// historical-unprefixed alias) must keep working.
func TestLoad_ShellOperatorEnvFallback(t *testing.T) {
	cases := []struct {
		envName string
		envVal  string
		get     func(*ad_app.Config) any
		want    any
	}{
		{"SHELL_OPERATOR_HOOKS_DIR", "/so/hooks",
			func(c *ad_app.Config) any { return c.App.GlobalHooksDir }, "/so/hooks"},
		{"SHELL_OPERATOR_TMP_DIR", "/so/tmp",
			func(c *ad_app.Config) any { return c.App.TempDir }, "/so/tmp"},
		{"SHELL_OPERATOR_LISTEN_ADDRESS", "10.0.0.1",
			func(c *ad_app.Config) any { return c.App.ListenAddress }, "10.0.0.1"},
		{"SHELL_OPERATOR_LISTEN_PORT", "9999",
			func(c *ad_app.Config) any { return c.App.ListenPort }, "9999"},
		{"SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX", "so_",
			func(c *ad_app.Config) any { return c.App.PrometheusMetricsPrefix }, "so_"},
		{"SHELL_OPERATOR_NAMESPACE", "so-ns",
			func(c *ad_app.Config) any { return c.App.Namespace }, "so-ns"},
	}
	for _, tc := range cases {
		t.Run(tc.envName, func(t *testing.T) {
			t.Setenv(tc.envName, tc.envVal)

			cfg := ad_app.NewConfig()
			if err := Load(cfg); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := tc.get(cfg); got != tc.want {
				t.Fatalf("%s: got %v, want %v", tc.envName, got, tc.want)
			}
		})
	}
}

// TestLoad_AddonOperatorEnvWinsOverShellOperator pins the precedence rule:
// when both the SHELL_OPERATOR_* fallback and the corresponding
// ADDON_OPERATOR_*/historical unprefixed name are set, the
// addon-operator-side value wins. This matches upstream addon-operator's own
// applyShellOperatorEnv -> ParseEnv call order.
func TestLoad_AddonOperatorEnvWinsOverShellOperator(t *testing.T) {
	cases := []struct {
		shellOpEnv string
		aoEnv      string
		shellOpVal string
		aoVal      string
		get        func(*ad_app.Config) any
	}{
		{
			shellOpEnv: "SHELL_OPERATOR_HOOKS_DIR",
			aoEnv:      "GLOBAL_HOOKS_DIR",
			shellOpVal: "/so/hooks",
			aoVal:      "/ao/hooks",
			get:        func(c *ad_app.Config) any { return c.App.GlobalHooksDir },
		},
		{
			shellOpEnv: "SHELL_OPERATOR_TMP_DIR",
			aoEnv:      "ADDON_OPERATOR_TMP_DIR",
			shellOpVal: "/so/tmp",
			aoVal:      "/ao/tmp",
			get:        func(c *ad_app.Config) any { return c.App.TempDir },
		},
		{
			shellOpEnv: "SHELL_OPERATOR_LISTEN_ADDRESS",
			aoEnv:      "ADDON_OPERATOR_LISTEN_ADDRESS",
			shellOpVal: "10.0.0.1",
			aoVal:      "127.0.0.1",
			get:        func(c *ad_app.Config) any { return c.App.ListenAddress },
		},
		{
			shellOpEnv: "SHELL_OPERATOR_LISTEN_PORT",
			aoEnv:      "ADDON_OPERATOR_LISTEN_PORT",
			shellOpVal: "9999",
			aoVal:      "4222",
			get:        func(c *ad_app.Config) any { return c.App.ListenPort },
		},
		{
			shellOpEnv: "SHELL_OPERATOR_PROMETHEUS_METRICS_PREFIX",
			aoEnv:      "ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX",
			shellOpVal: "so_",
			aoVal:      "ao_",
			get:        func(c *ad_app.Config) any { return c.App.PrometheusMetricsPrefix },
		},
		{
			shellOpEnv: "SHELL_OPERATOR_NAMESPACE",
			aoEnv:      "ADDON_OPERATOR_NAMESPACE",
			shellOpVal: "so-ns",
			aoVal:      "ao-ns",
			get:        func(c *ad_app.Config) any { return c.App.Namespace },
		},
	}
	for _, tc := range cases {
		t.Run(tc.shellOpEnv+"_vs_"+tc.aoEnv, func(t *testing.T) {
			t.Setenv(tc.shellOpEnv, tc.shellOpVal)
			t.Setenv(tc.aoEnv, tc.aoVal)

			cfg := ad_app.NewConfig()
			if err := Load(cfg); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := tc.get(cfg); got != tc.aoVal {
				t.Fatalf("%s wins over %s: got %v, want %v",
					tc.aoEnv, tc.shellOpEnv, got, tc.aoVal)
			}
		})
	}
}

// TestLoad_ThenApplyConfig_SyncsDebugSocketGlobals pins the contract that the
// deckhouse-controller main() flow — envconfig.Load(cfg) followed by
// ad_app.ApplyConfig(cfg) and direct assignment of sh_debug.DefaultSocketPath
// — propagates DEBUG_UNIX_SOCKET into the addon-operator and shell-operator
// package-level globals consulted by debug sub-commands (queue, hook, global,
// module, raw) when NewAddonOperator is not invoked (i.e. for any CLI flow
// other than `start`). A regression here brings back
// `debug socket '/var/run/shell-operator/debug.socket' is not exists`.
func TestLoad_ThenApplyConfig_SyncsDebugSocketGlobals(t *testing.T) {
	const want = "/tmp/shell-operator-debug.socket"

	prevAd := ad_app.DebugUnixSocket
	prevSh := sh_debug.DefaultSocketPath
	t.Cleanup(func() {
		ad_app.DebugUnixSocket = prevAd
		sh_debug.DefaultSocketPath = prevSh
	})

	ad_app.DebugUnixSocket = "/stale/addon-operator.socket"
	sh_debug.DefaultSocketPath = "/stale/shell-operator.socket"

	t.Setenv("DEBUG_UNIX_SOCKET", want)

	cfg := ad_app.NewConfig()
	if err := Load(cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Debug.UnixSocket != want {
		t.Fatalf("cfg.Debug.UnixSocket: got %q, want %q", cfg.Debug.UnixSocket, want)
	}

	ad_app.ApplyConfig(cfg)
	sh_debug.DefaultSocketPath = cfg.Debug.UnixSocket

	if ad_app.DebugUnixSocket != want {
		t.Errorf("ad_app.DebugUnixSocket: got %q, want %q (ad_app.ApplyConfig must mirror cfg into addon-operator global)",
			ad_app.DebugUnixSocket, want)
	}
	if sh_debug.DefaultSocketPath != want {
		t.Errorf("sh_debug.DefaultSocketPath: got %q, want %q (assignment must mirror cfg into shell-operator global so queue/hook/raw CLI dial the right path)",
			sh_debug.DefaultSocketPath, want)
	}
}
