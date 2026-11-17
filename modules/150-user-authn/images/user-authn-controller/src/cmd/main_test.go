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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"user-authn-controller/internal/controller/dexauthenticator"
	"user-authn-controller/internal/controller/useraccount"
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
	assertDropsManagedFields(t, opts)
}

// assertDropsManagedFields checks the default transform of the cache: every kind without a transform
// of its own (User, Group, DexClient, UserAccount, the Service, Ingress and Pod of the
// DexAuthenticator reconcilers) is stored without managedFields, as nil and not as an empty list, so
// that an object written back from the cache keeps them on the API server.
func assertDropsManagedFields(t *testing.T, opts manager.Options) {
	t.Helper()

	if opts.Cache.DefaultTransform == nil {
		t.Fatal("Cache.DefaultTransform is nil, want a transform that drops managedFields")
	}
	fields := []metav1.ManagedFieldsEntry{{Manager: "user-authn-controller", Operation: metav1.ManagedFieldsOperationUpdate}}
	user := &unstructured.Unstructured{}
	user.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "User"})
	user.SetName("jane")
	user.SetManagedFields(fields)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", ManagedFields: fields}}
	for _, obj := range []client.Object{user, pod} {
		got, err := opts.Cache.DefaultTransform(obj)
		if err != nil {
			t.Fatalf("transform %T: %v", obj, err)
		}
		stored, ok := got.(client.Object)
		if !ok {
			t.Fatalf("transform %T returned %T", obj, got)
		}
		if stored.GetManagedFields() != nil || stored.GetName() == "" {
			t.Errorf("%T stored as %+v, want its name and no managedFields", obj, stored)
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

	dex, err := newDexClientManagerOptions(runtime.NewScheme(), nil)
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
	assertDropsManagedFields(t, dex)
}

// With the shared dex-authenticator instances the dexclient command also caches what the
// DexAuthenticator reconcilers watch, and of each kind only what they need: the Services, Ingresses
// and HTTPRoutes the controller wrote, the pods of the instances, the ConfigMap of the settings.
func TestManagerOptionsOfTheDexAuthenticatorReconcilers(t *testing.T) {
	scopes := func(gatewayAPI bool) map[string]cache.ByObject {
		opts, err := newDexClientManagerOptions(runtime.NewScheme(), &dexauthenticator.Options{GatewayAPI: gatewayAPI})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]cache.ByObject{}
		for obj, byObject := range opts.Cache.ByObject {
			kind := obj.GetObjectKind().GroupVersionKind().Kind
			if kind == "" {
				kind = reflect.TypeOf(obj).Elem().Name()
			}
			out[kind] = byObject
		}
		return out
	}

	without := scopes(false)
	if _, cached := without["HTTPRoute"]; cached {
		t.Error("HTTPRoutes are cached without the Gateway API: the informer of a kind the cluster does not have never syncs")
	}
	with := scopes(true)
	for _, kind := range []string{"Service", "Ingress", "HTTPRoute", "Secret"} {
		selector := with[kind].Namespaces[cache.AllNamespaces].LabelSelector
		if selector == nil || selector.String() != "app.kubernetes.io/managed-by=user-authn-controller" {
			t.Errorf("%s cache selector = %v, want the objects of the controller only", kind, selector)
		}
	}
	if pods := with["Pod"].Namespaces["d8-user-authn"]; len(with["Pod"].Namespaces) != 1 || pods.LabelSelector == nil || pods.LabelSelector.String() != "app=shared-dex-authenticator" {
		t.Errorf("Pod cache = %+v, want the pods of the instances in d8-user-authn", with["Pod"])
	}
	if settings := with["ConfigMap"].Namespaces["d8-user-authn"]; len(with["ConfigMap"].Namespaces) != 1 || settings.FieldSelector == nil || settings.FieldSelector.String() != "metadata.name=shared-dex-authenticator" {
		t.Errorf("ConfigMap cache = %+v, want the ConfigMap of the settings", with["ConfigMap"])
	}
}

func TestSharedDexAuthenticatorOptions(t *testing.T) {
	if got := sharedDexAuthenticatorOptions(); got != nil {
		t.Errorf("without the variable: %+v, want the reconcilers off", got)
	}
	t.Setenv(sharedDexAuthenticatorEnv, "true")
	if got := sharedDexAuthenticatorOptions(); got == nil || got.GatewayAPI {
		t.Errorf("with the variable: %+v, want the reconcilers on without the Gateway API", got)
	}
	t.Setenv(gatewayAPIEnv, "true")
	if got := sharedDexAuthenticatorOptions(); got == nil || !got.GatewayAPI {
		t.Errorf("with both variables: %+v, want the Gateway API on", got)
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

func TestPasswordHistoryLimit(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    uint64
		wantErr bool
	}{
		{value: "", want: 0},
		{value: "0", want: 0},
		{value: "5", want: 5},
		{value: "-1", wantErr: true},
		{value: "five", wantErr: true},
	} {
		got, err := passwordHistoryLimit(tc.value)
		if tc.wantErr {
			if err == nil {
				t.Errorf("passwordHistoryLimit(%q) = %v, want an error", tc.value, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("passwordHistoryLimit(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
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

func TestOrphanedIdentityRetention(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{value: "", want: 30 * 24 * time.Hour},
		{value: "5m", want: 5 * time.Minute},
		{value: "0s", wantErr: true},
		{value: "-1h", wantErr: true},
		{value: "a month", wantErr: true},
	} {
		got, err := orphanedIdentityRetention(tc.value)
		if tc.wantErr {
			if err == nil {
				t.Errorf("orphanedIdentityRetention(%q) = %v, want an error", tc.value, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("orphanedIdentityRetention(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}
}

func TestRefreshTokenLifetimes(t *testing.T) {
	for _, tc := range []struct {
		absolute, idle string
		want           useraccount.RefreshTokenLifetimes
		wantErr        bool
	}{
		{want: useraccount.RefreshTokenLifetimes{AbsoluteLifetime: 4380 * time.Hour, ValidIfNotUsedFor: 2190 * time.Hour}},
		{absolute: "720h", idle: "24h", want: useraccount.RefreshTokenLifetimes{AbsoluteLifetime: 720 * time.Hour, ValidIfNotUsedFor: 24 * time.Hour}},
		{absolute: "0s", idle: "1h", want: useraccount.RefreshTokenLifetimes{ValidIfNotUsedFor: time.Hour}},
		{absolute: "-1h", wantErr: true},
		{idle: "a day", wantErr: true},
	} {
		got, err := refreshTokenLifetimes(tc.absolute, tc.idle)
		if tc.wantErr {
			if err == nil {
				t.Errorf("refreshTokenLifetimes(%q, %q) = %v, want an error", tc.absolute, tc.idle, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("refreshTokenLifetimes(%q, %q) = %v, %v; want %v", tc.absolute, tc.idle, got, err, tc.want)
		}
	}
}
