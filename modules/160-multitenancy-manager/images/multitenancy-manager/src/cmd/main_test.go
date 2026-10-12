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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
