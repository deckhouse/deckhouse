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
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"controller/apis/deckhouse.io/v1alpha3"
)

// The startup wait lists pods and ReplicaSets through the uncached API reader of the manager, which
// uses this scheme. A kind missing from it fails every check, and the wait holds each start of the
// controllers for its whole timeout.
func TestNewScheme_KnowsWhatTheStartupWaitReads(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("build the scheme: %v", err)
	}
	for _, list := range []client.ObjectList{&corev1.PodList{}, &appsv1.ReplicaSetList{}} {
		if _, err := apiutil.GVKForObject(list, scheme); err != nil {
			t.Errorf("the scheme cannot read %T: %v", list, err)
		}
	}
}

// The cache of the controller keeps no managedFields: nothing reads them from it (the Helm takeover
// reads objects as unstructured, which bypasses the cache). The types with a transform of their own
// keep it, and every other type gets the default.
func TestNewManagerOptions_StripsManagedFieldsByDefault(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("build the scheme: %v", err)
	}
	opts := newManagerOptions(scheme, nil)

	if opts.Cache.DefaultTransform == nil {
		t.Fatal("Cache.DefaultTransform is nil, want a transform that drops managedFields")
	}
	// ByObject is keyed by pointers, so the entry is found by its type.
	serviceTransform := false
	for obj, byObject := range opts.Cache.ByObject {
		if _, ok := obj.(*corev1.Service); ok {
			serviceTransform = byObject.Transform != nil
		}
	}
	if !serviceTransform {
		t.Error("the transform of Service is gone")
	}

	fields := []metav1.ManagedFieldsEntry{{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationUpdate}}
	objects := []client.Object{
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "admins", Namespace: "p", ManagedFields: fields},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "admin"},
		},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "admin", ManagedFields: fields}},
		&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "q", Namespace: "p", ManagedFields: fields}},
		&admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "w", ManagedFields: fields}},
		// The grant webhooks keep every namespace and project on each replica.
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "p", ManagedFields: fields}},
		&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "p", ManagedFields: fields}},
	}
	for _, obj := range objects {
		got, err := opts.Cache.DefaultTransform(obj)
		if err != nil {
			t.Fatalf("transform %T: %v", obj, err)
		}
		stored, ok := got.(client.Object)
		if !ok {
			t.Fatalf("transform %T returned %T, want a client.Object", obj, got)
		}
		// nil, not an empty list: an object written back with an empty list would reset its
		// managed fields on the API server.
		if stored.GetManagedFields() != nil {
			t.Errorf("%T keeps managedFields %v, want nil", obj, stored.GetManagedFields())
		}
		if stored.GetName() != obj.GetName() {
			t.Errorf("%T lost its name: %q", obj, stored.GetName())
		}
	}
	if rb := objects[0].(*rbacv1.RoleBinding); rb.RoleRef.Name != "admin" {
		t.Errorf("RoleBinding roleRef = %+v, want it kept", rb.RoleRef)
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
