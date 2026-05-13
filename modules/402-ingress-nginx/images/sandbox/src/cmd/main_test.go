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
	"reflect"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestNormalizeSandboxArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "drops leading separator for child argv",
			in:   []string{"--", "/usr/local/nginx/sbin/nginx", "-c", "/tmp/nginx/nginx-cfg123"},
			want: []string{"/usr/local/nginx/sbin/nginx", "-c", "/tmp/nginx/nginx-cfg123"},
		},
		{
			name: "keeps plain argv intact",
			in:   []string{"/usr/local/nginx/sbin/nginx", "-c", "/tmp/nginx/nginx-cfg123"},
			want: []string{"/usr/local/nginx/sbin/nginx", "-c", "/tmp/nginx/nginx-cfg123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSandboxArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected argv, got %v want %v", got, tt.want)
			}
		})
	}
}

func TestParseSandboxMode(t *testing.T) {
	tests := []struct {
		name     string
		in       []string
		wantMode sandboxMode
		wantArgv []string
	}{
		{
			name:     "isolated process mode",
			in:       []string{"--isolated-process", "--", "/usr/local/nginx/sbin/nginx"},
			wantMode: sandboxModeIsolatedProcess,
			wantArgv: []string{"--", "/usr/local/nginx/sbin/nginx"},
		},
		{
			name:     "isolated process child mode",
			in:       []string{"--isolated-process-child", "--", "/usr/local/nginx/sbin/nginx"},
			wantMode: sandboxModeIsolatedProcessChild,
			wantArgv: []string{"--", "/usr/local/nginx/sbin/nginx"},
		},
		{
			name:     "default mode",
			in:       []string{"--", "/usr/local/nginx/sbin/nginx"},
			wantMode: sandboxModeDefault,
			wantArgv: []string{"--", "/usr/local/nginx/sbin/nginx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotArgv := parseSandboxMode(tt.in)
			if gotMode != tt.wantMode {
				t.Fatalf("unexpected mode, got %v want %v", gotMode, tt.wantMode)
			}
			if !reflect.DeepEqual(gotArgv, tt.wantArgv) {
				t.Fatalf("unexpected argv, got %v want %v", gotArgv, tt.wantArgv)
			}
		})
	}
}

func TestNewIsolatedProcessChildCmd(t *testing.T) {
	cmd := newIsolatedProcessChildCmd("/usr/bin/sandbox", []string{"/usr/local/nginx/sbin/nginx", "-c", "/tmp/nginx/nginx-cfg123"}, 64535, 64535)

	if cmd.Path != "/usr/bin/sandbox" {
		t.Fatalf("unexpected path, got %q", cmd.Path)
	}

	wantArgs := []string{
		"/usr/bin/sandbox",
		"--isolated-process-child",
		"--",
		"/usr/local/nginx/sbin/nginx",
		"-c",
		"/tmp/nginx/nginx-cfg123",
	}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("unexpected args, got %v want %v", cmd.Args, wantArgs)
	}

	wantSys := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{{
			ContainerID: 64535,
			HostID:      64535,
			Size:        1,
		}},
		GidMappings: []syscall.SysProcIDMap{{
			ContainerID: 64535,
			HostID:      64535,
			Size:        1,
		}},
		GidMappingsEnableSetgroups: false,
		AmbientCaps: []uintptr{
			unix.CAP_NET_BIND_SERVICE,
			unix.CAP_SYS_CHROOT,
		},
	}

	assertExecCmdSysProcAttrEqual(t, cmd.SysProcAttr, wantSys)
}

func assertExecCmdSysProcAttrEqual(t *testing.T, got, want *syscall.SysProcAttr) {
	t.Helper()

	if got == nil {
		t.Fatal("expected SysProcAttr to be set")
	}

	if got.Cloneflags != want.Cloneflags {
		t.Fatalf("unexpected Cloneflags, got %x want %x", got.Cloneflags, want.Cloneflags)
	}
	if !reflect.DeepEqual(got.UidMappings, want.UidMappings) {
		t.Fatalf("unexpected UidMappings, got %+v want %+v", got.UidMappings, want.UidMappings)
	}
	if !reflect.DeepEqual(got.GidMappings, want.GidMappings) {
		t.Fatalf("unexpected GidMappings, got %+v want %+v", got.GidMappings, want.GidMappings)
	}
	if got.GidMappingsEnableSetgroups != want.GidMappingsEnableSetgroups {
		t.Fatalf("unexpected GidMappingsEnableSetgroups, got %v want %v", got.GidMappingsEnableSetgroups, want.GidMappingsEnableSetgroups)
	}
	if !reflect.DeepEqual(got.AmbientCaps, want.AmbientCaps) {
		t.Fatalf("unexpected AmbientCaps, got %v want %v", got.AmbientCaps, want.AmbientCaps)
	}
}
