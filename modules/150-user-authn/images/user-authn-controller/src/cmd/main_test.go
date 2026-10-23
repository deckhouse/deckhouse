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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestNewManagerOptions(t *testing.T) {
	// Leader election does not depend on HA any more: without HA the Deployment keeps its default
	// rolling update, so two pods run during every version change, and two of them writing the
	// same UserAccounts and DexProviderChecks would fight over them.
	opts, err := newManagerOptions(runtime.NewScheme())
	if err != nil {
		t.Fatal(err)
	}

	if !opts.LeaderElection || opts.LeaderElectionID != controllerName || opts.LeaderElectionNamespace != leaderElectionNamespace {
		t.Fatalf("leader election must be on in the module namespace, got %+v", opts)
	}
	if !opts.LeaderElectionReleaseOnCancel {
		t.Error("the lease must be released on shutdown, or the next pod waits out the lease duration")
	}
	// Losing the lease exits the process, so the lease has to outlive a short API-server absence.
	if opts.LeaseDuration == nil || *opts.LeaseDuration < 30*time.Second {
		t.Errorf("LeaseDuration = %v, want at least 30s", opts.LeaseDuration)
	}
	if opts.RenewDeadline == nil || opts.LeaseDuration == nil || *opts.RenewDeadline >= *opts.LeaseDuration {
		t.Errorf("RenewDeadline = %v must be shorter than LeaseDuration = %v", opts.RenewDeadline, opts.LeaseDuration)
	}
	if opts.RetryPeriod == nil || opts.RenewDeadline == nil || *opts.RetryPeriod*2 > *opts.RenewDeadline {
		t.Errorf("RetryPeriod = %v must leave several tries within RenewDeadline = %v", opts.RetryPeriod, opts.RenewDeadline)
	}
	if opts.Client.Cache == nil || len(opts.Client.Cache.DisableFor) != 1 {
		t.Fatalf("client cache = %+v, want Secrets read past the cache", opts.Client.Cache)
	}
	if _, ok := opts.Client.Cache.DisableFor[0].(*corev1.Secret); !ok {
		t.Errorf("DisableFor = %T, want *corev1.Secret: reading a Secret from the cache would start an informer holding Secret data", opts.Client.Cache.DisableFor[0])
	}
	if opts.HealthProbeBindAddress != ":9090" {
		t.Errorf("HealthProbeBindAddress = %q, want %q", opts.HealthProbeBindAddress, ":9090")
	}
	if opts.Metrics.BindAddress != ":9091" {
		t.Errorf("Metrics.BindAddress = %q, want %q", opts.Metrics.BindAddress, ":9091")
	}
	if opts.WebhookServer != nil {
		t.Errorf("WebhookServer = %v, want nil", opts.WebhookServer)
	}
	// The revocation feed authenticates DexAuthenticators from a cache of their clients without
	// secrets; every other cached kind of Dex is scoped to the Dex namespace too.
	dexKinds := map[string]cache.ByObject{}
	for obj, byObject := range opts.Cache.ByObject {
		if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Group == "dex.coreos.com" {
			dexKinds[gvk.Kind] = byObject
		}
	}
	for _, kind := range []string{"Password", "OfflineSessions", "RefreshToken", "UserIdentity", "AuthSession", "Revocation", "OAuth2Client"} {
		byObject, ok := dexKinds[kind]
		if !ok {
			t.Errorf("no cache config for %s", kind)
			continue
		}
		if byObject.Transform == nil || len(byObject.Namespaces) != 1 {
			t.Errorf("cache of %s = %+v, want a transform and one namespace", kind, byObject)
		}
	}
}

// The DexClient reconciler is the only writer of Secrets, so it runs under its own ServiceAccount:
// the main controller caches no Secret, and the dexclient command caches only the labelled Secrets
// and the OAuth2Clients, under a lease of its own.
func TestManagerOptionsSplitTheSecretWriter(t *testing.T) {
	main, err := newManagerOptions(runtime.NewScheme())
	if err != nil {
		t.Fatal(err)
	}
	for obj := range main.Cache.ByObject {
		if _, ok := obj.(*corev1.Secret); ok {
			t.Error("the main controller must not cache Secrets: it does not run the DexClient reconciler")
		}
	}

	dex, err := newDexClientManagerOptions(runtime.NewScheme())
	if err != nil {
		t.Fatal(err)
	}
	if !dex.LeaderElection || dex.LeaderElectionID != dexClientControllerName || dex.LeaderElectionID == controllerName {
		t.Fatalf("dexclient leader election = %v %q, want a lease of its own", dex.LeaderElection, dex.LeaderElectionID)
	}
	if dex.Client.Cache == nil || len(dex.Client.Cache.DisableFor) != 1 {
		t.Fatalf("client cache = %+v, want Secrets read past the cache", dex.Client.Cache)
	}
	var kinds []string
	for obj, byObject := range dex.Cache.ByObject {
		switch o := obj.(type) {
		case *corev1.Secret:
			kinds = append(kinds, "Secret")
			if byObject.Namespaces[cache.AllNamespaces].LabelSelector == nil {
				t.Error("the Secret cache must be scoped by label")
			}
		default:
			kinds = append(kinds, o.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	slices.Sort(kinds)
	if !slices.Equal(kinds, []string{"OAuth2Client", "Secret"}) {
		t.Errorf("dexclient caches %v, want only OAuth2Client and Secret", kinds)
	}
}

func TestLogLevel(t *testing.T) {
	cases := map[string]zapcore.Level{"": zapcore.InfoLevel, "debug": zapcore.DebugLevel, "warn": zapcore.WarnLevel, "ERROR": zapcore.ErrorLevel, "loud": zapcore.InfoLevel}
	for in, want := range cases {
		if got := logLevel(in); got != want {
			t.Errorf("logLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// Every line of controller-runtime and of klog is JSON with a named level: with debug, V(1) is a debug
// message and deeper levels are dropped; with the default, both are.
func TestSetupLogging(t *testing.T) {
	for name, tc := range map[string]struct {
		level     string
		wantDebug bool
	}{
		"default": {level: "", wantDebug: false},
		"debug":   {level: "debug", wantDebug: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(logLevelEnv, tc.level)
			var out bytes.Buffer
			logger := setupLogging(zap.WriteTo(&out))
			t.Cleanup(klog.ClearLogger)

			logger.Info("info message")
			logger.V(1).Info("v1 message")
			logger.V(2).Info("v2 message")
			logger.V(4).Info("v4 message")
			klog.InfoS("klog info message")
			klog.ErrorS(errors.New("lease conflict"), "klog error message")

			got := map[string]string{}
			for line := range strings.Lines(out.String()) {
				var entry struct {
					Level string `json:"level"`
					Msg   string `json:"msg"`
				}
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					t.Fatalf("not a JSON line %q: %v", line, err)
				}
				got[entry.Msg] = entry.Level
			}
			want := map[string]string{"info message": "info", "klog info message": "info", "klog error message": "error"}
			if tc.wantDebug {
				want["v1 message"] = "debug"
			}
			if !maps.Equal(got, want) {
				t.Fatalf("levels = %v, want %v", got, want)
			}
		})
	}
}

func TestRevocationRetention(t *testing.T) {
	for _, tc := range []struct {
		value      string
		configured string
		want       time.Duration
		wantErr    bool
	}{
		{value: "", want: 70 * time.Minute},
		{value: "10m", want: 70 * time.Minute},
		{value: "2h30m10s", want: 3*time.Hour + 30*time.Minute + 10*time.Second},
		{value: "0s", wantErr: true},
		{value: "ten minutes", wantErr: true},
		// A configured retention keeps the revocations longer, never shorter.
		{value: "10m", configured: "24h", want: 24 * time.Hour},
		{value: "2h", configured: "30m", want: 3 * time.Hour},
		{value: "10m", configured: "a day", wantErr: true},
	} {
		got, err := revocationRetention(tc.value, tc.configured)
		if tc.wantErr {
			if err == nil {
				t.Errorf("revocationRetention(%q, %q) = %v, want an error", tc.value, tc.configured, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("revocationRetention(%q, %q) = %v, %v; want %v", tc.value, tc.configured, got, err, tc.want)
		}
	}
}

func TestOperationRetention(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 24 * time.Hour},
		{value: "168h", want: 168 * time.Hour},
		{value: "0s", wantErr: true},
		{value: "a week", wantErr: true},
	} {
		got, err := operationRetention(tc.value)
		if tc.wantErr {
			if err == nil {
				t.Errorf("operationRetention(%q) = %v, want an error", tc.value, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("operationRetention(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
}

func TestMinDeviceAge(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 24 * time.Hour},
		{value: "1h30m", want: 90 * time.Minute},
		{value: "0s", want: 0},
		{value: "-1h", wantErr: true},
		{value: "a day", wantErr: true},
	} {
		got, err := minDeviceAge(tc.value)
		if tc.wantErr {
			if err == nil {
				t.Errorf("minDeviceAge(%q) = %v, want an error", tc.value, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("minDeviceAge(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
}
