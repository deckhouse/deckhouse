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

package dvpcsidriver

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func TestLogGRPC_SanitizesNodeStageVolumeMountFlags(t *testing.T) {
	sentinel := "password=CSI_MOUNT_FLAGS_SENTINEL_STAGE"
	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "volume-stage",
		StagingTargetPath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/volume-stage/globalmount",
		VolumeCapability:  mountVolumeCapability(sentinel),
	}

	logs := captureVerboseKlogOutput(t, func() {
		_, err := logGRPC(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodeStageVolume"}, func(_ context.Context, got interface{}) (interface{}, error) {
			gotReq, ok := got.(*csi.NodeStageVolumeRequest)
			if !ok {
				t.Fatalf("request type = %T, want *csi.NodeStageVolumeRequest", got)
			}
			if !contains(gotReq.GetVolumeCapability().GetMount().GetMountFlags(), sentinel) {
				t.Fatalf("handler request mount flags = %v, want %q", gotReq.GetVolumeCapability().GetMount().GetMountFlags(), sentinel)
			}
			return &csi.NodeStageVolumeResponse{}, nil
		})
		if err != nil {
			t.Fatalf("logGRPC() error = %v", err)
		}
	})

	assertLogPathsExecuted(t, logs)
	assertLogsDoNotContain(t, logs, sentinel)
	if !contains(req.GetVolumeCapability().GetMount().GetMountFlags(), sentinel) {
		t.Fatalf("original request mount flags = %v, want %q", req.GetVolumeCapability().GetMount().GetMountFlags(), sentinel)
	}
}

func TestLogGRPC_SanitizesNodePublishVolumeMountFlags(t *testing.T) {
	sentinel := "password=CSI_MOUNT_FLAGS_SENTINEL_PUBLISH"
	req := &csi.NodePublishVolumeRequest{
		VolumeId:          "volume-publish",
		StagingTargetPath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/volume-publish/globalmount",
		TargetPath:        "/var/lib/kubelet/pods/pod/volumes/kubernetes.io~csi/volume-publish/mount",
		VolumeCapability:  mountVolumeCapability(sentinel),
	}

	logs := captureVerboseKlogOutput(t, func() {
		_, err := logGRPC(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodePublishVolume"}, func(_ context.Context, got interface{}) (interface{}, error) {
			gotReq, ok := got.(*csi.NodePublishVolumeRequest)
			if !ok {
				t.Fatalf("request type = %T, want *csi.NodePublishVolumeRequest", got)
			}
			if !contains(gotReq.GetVolumeCapability().GetMount().GetMountFlags(), sentinel) {
				t.Fatalf("handler request mount flags = %v, want %q", gotReq.GetVolumeCapability().GetMount().GetMountFlags(), sentinel)
			}
			return &csi.NodePublishVolumeResponse{}, nil
		})
		if err != nil {
			t.Fatalf("logGRPC() error = %v", err)
		}
	})

	assertLogPathsExecuted(t, logs)
	assertLogsDoNotContain(t, logs, sentinel)
	if !contains(req.GetVolumeCapability().GetMount().GetMountFlags(), sentinel) {
		t.Fatalf("original request mount flags = %v, want %q", req.GetVolumeCapability().GetMount().GetMountFlags(), sentinel)
	}
}

func TestSanitizeGRPCLogValue_SanitizesRepeatedVolumeCapabilities(t *testing.T) {
	sentinel := "password=CSI_MOUNT_FLAGS_SENTINEL_REPEATED"
	req := &csi.ValidateVolumeCapabilitiesRequest{
		VolumeId: "volume-validate",
		VolumeCapabilities: []*csi.VolumeCapability{
			mountVolumeCapability(sentinel),
			mountVolumeCapability("ro"),
		},
	}

	sanitized := sanitizeGRPCLogValue(req)
	data, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	assertLogsDoNotContain(t, string(data), sentinel)
	if !contains(req.GetVolumeCapabilities()[0].GetMount().GetMountFlags(), sentinel) {
		t.Fatalf("original request mount flags = %v, want %q", req.GetVolumeCapabilities()[0].GetMount().GetMountFlags(), sentinel)
	}
}

func TestLogGRPCJson_SanitizesCSISecrets(t *testing.T) {
	mountFlagSentinel := "password=CSI_MOUNT_FLAGS_SENTINEL_JSON"
	secretSentinel := "CSI_SECRET_SENTINEL_JSON"
	req := &csi.NodeStageVolumeRequest{
		VolumeId:          "volume-json",
		StagingTargetPath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/volume-json/globalmount",
		VolumeCapability:  mountVolumeCapability(mountFlagSentinel),
		Secrets:           map[string]string{"password": secretSentinel},
	}

	logs := captureVerboseKlogOutput(t, func() {
		logGRPCJson("/csi.v1.Node/NodeStageVolume", req, &csi.NodeStageVolumeResponse{}, nil)
	})

	if !strings.Contains(logs, "gRPCCall:") {
		t.Fatalf("logs do not contain gRPCCall: %s", logs)
	}
	assertLogsDoNotContain(t, logs, mountFlagSentinel)
	assertLogsDoNotContain(t, logs, secretSentinel)
	if !contains(req.GetVolumeCapability().GetMount().GetMountFlags(), mountFlagSentinel) {
		t.Fatalf("original request mount flags = %v, want %q", req.GetVolumeCapability().GetMount().GetMountFlags(), mountFlagSentinel)
	}
	if req.GetSecrets()["password"] != secretSentinel {
		t.Fatalf("original request secret = %q, want %q", req.GetSecrets()["password"], secretSentinel)
	}
}

func TestLogGRPC_PreservesUnknownEnumValue(t *testing.T) {
	req := &csi.NodePublishVolumeRequest{
		VolumeId:          "volume-unknown-enum",
		StagingTargetPath: "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/volume-unknown-enum/globalmount",
		TargetPath:        "/var/lib/kubelet/pods/pod/volumes/kubernetes.io~csi/volume-unknown-enum/mount",
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_Mode(99)},
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
		},
	}
	handlerCalled := false

	logs := captureVerboseKlogOutput(t, func() {
		_, err := logGRPC(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: "/csi.v1.Node/NodePublishVolume"}, func(_ context.Context, got interface{}) (interface{}, error) {
			handlerCalled = true
			gotReq, ok := got.(*csi.NodePublishVolumeRequest)
			if !ok {
				t.Fatalf("request type = %T, want *csi.NodePublishVolumeRequest", got)
			}
			if gotReq.GetVolumeCapability().GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_Mode(99) {
				t.Fatalf("handler access mode = %v, want 99", gotReq.GetVolumeCapability().GetAccessMode().GetMode())
			}
			return &csi.NodePublishVolumeResponse{}, nil
		})
		if err != nil {
			t.Fatalf("logGRPC() error = %v", err)
		}
	})

	if !handlerCalled {
		t.Fatal("handler was not called")
	}
	assertLogPathsExecuted(t, logs)
	if !strings.Contains(logs, "99") {
		t.Fatalf("logs do not contain unknown enum value 99: %s", logs)
	}
}

func captureVerboseKlogOutput(t *testing.T, fn func()) string {
	t.Helper()
	state := klog.CaptureState()
	defer state.Restore()

	flags := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	flags.SetOutput(new(bytes.Buffer))
	klog.InitFlags(flags)
	if err := flags.Set("v", "5"); err != nil {
		t.Fatalf("failed to set klog verbosity: %v", err)
	}

	buf := new(bytes.Buffer)
	klog.SetOutput(buf)
	klog.LogToStderr(false)

	fn()
	klog.Flush()

	return buf.String()
}

func mountVolumeCapability(mountFlags ...string) *csi.VolumeCapability {
	return &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4", MountFlags: mountFlags}},
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func assertLogPathsExecuted(t *testing.T, logs string) {
	t.Helper()
	for _, want := range []string{"GRPC request:", "gRPCCall:"} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs do not contain %q: %s", want, logs)
		}
	}
}

func assertLogsDoNotContain(t *testing.T, logs, forbidden string) {
	t.Helper()
	if strings.Contains(logs, forbidden) {
		t.Fatalf("logs contain %q: %s", forbidden, logs)
	}
}
