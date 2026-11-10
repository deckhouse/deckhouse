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

package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	testingexec "k8s.io/utils/exec/testing"
)

func TestMaxVolumesPerNode(t *testing.T) {
	for _, tc := range []struct {
		staticDisks int
		want        int64
	}{
		{staticDisks: 0, want: 16},
		{staticDisks: 1, want: 15},
		{staticDisks: 3, want: 13},
		{staticDisks: 15, want: 1},
		{staticDisks: 16, want: 1},
		{staticDisks: 20, want: 1},
	} {
		if got := maxVolumesPerNode(tc.staticDisks); got != tc.want {
			t.Errorf("maxVolumesPerNode(%d) = %d, want %d", tc.staticDisks, got, tc.want)
		}
	}
}

type execCall struct {
	cmd    string
	args   []string
	output []byte
	err    error
}

type recordedExec struct {
	*testingexec.FakeExec
	wantCalls [][]string
}

type mountCall struct {
	source           string
	target           string
	fsType           string
	options          []string
	sensitiveOptions []string
}

type recordingMounter struct {
	mounts               []mount.MountPoint
	mountCalls           []mountCall
	unmountCalls         []string
	mountErr             error
	unmountErr           error
	safeSkip             bool
	likelyNotMountPoints map[string]bool
}

func (m *recordingMounter) Mount(source string, target string, fsType string, options []string) error {
	return m.MountSensitive(source, target, fsType, options, nil)
}

func (m *recordingMounter) MountSensitive(source string, target string, fsType string, options []string, sensitiveOptions []string) error {
	callOptions := append([]string(nil), options...)
	callSensitiveOptions := append([]string(nil), sensitiveOptions...)
	m.mountCalls = append(m.mountCalls, mountCall{source: source, target: target, fsType: fsType, options: callOptions, sensitiveOptions: callSensitiveOptions})
	if m.mountErr != nil {
		return m.mountErr
	}
	mountOptions := append([]string(nil), callOptions...)
	mountOptions = append(mountOptions, callSensitiveOptions...)
	m.mounts = append(m.mounts, mount.MountPoint{Device: source, Path: target, Type: fsType, Opts: mountOptions})
	return nil
}

func (m *recordingMounter) MountSensitiveWithoutSystemd(source string, target string, fsType string, options []string, sensitiveOptions []string) error {
	return m.MountSensitive(source, target, fsType, options, sensitiveOptions)
}

func (m *recordingMounter) MountSensitiveWithoutSystemdWithMountFlags(source string, target string, fsType string, options []string, sensitiveOptions []string, _ []string) error {
	return m.MountSensitive(source, target, fsType, options, sensitiveOptions)
}

func (m *recordingMounter) Unmount(target string) error {
	m.unmountCalls = append(m.unmountCalls, target)
	if m.unmountErr != nil {
		return m.unmountErr
	}
	kept := m.mounts[:0]
	for _, mp := range m.mounts {
		if mp.Path != target {
			kept = append(kept, mp)
		}
	}
	m.mounts = kept
	return nil
}

func (m *recordingMounter) List() ([]mount.MountPoint, error) {
	return append([]mount.MountPoint{}, m.mounts...), nil
}

func (m *recordingMounter) IsLikelyNotMountPoint(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return true, err
	}
	if notMountPoint, ok := m.likelyNotMountPoints[path]; ok {
		return notMountPoint, nil
	}
	for _, mp := range m.mounts {
		if mp.Path == path {
			return false, nil
		}
	}
	return true, nil
}

func (m *recordingMounter) CanSafelySkipMountPointCheck() bool {
	return m.safeSkip
}

func (m *recordingMounter) IsMountPoint(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	for _, mp := range m.mounts {
		if mp.Path == path {
			return true, nil
		}
	}
	return false, nil
}

func (m *recordingMounter) GetMountRefs(path string) ([]string, error) {
	refs := []string{}
	for _, mp := range m.mounts {
		if mp.Device == path && mp.Path != path {
			refs = append(refs, mp.Path)
		}
	}
	return refs, nil
}

func TestNodeStageVolume_ExistingFilesystemMountsWithoutMkfs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fsType string
	}{
		{name: "ext4", fsType: "ext4"},
		{name: "xfs", fsType: "xfs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stagingTargetPath := t.TempDir()
			fakeExec := newFakeExec(
				execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=" + tc.fsType + "\n")},
				execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
			)
			mounter := &recordingMounter{}
			n := newTestNode(mounter, fakeExec)

			_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, tc.fsType))
			if err != nil {
				t.Fatalf("NodeStageVolume() error = %v", err)
			}
			assertExecConsumed(t, fakeExec)
			assertNoMkfs(t, fakeExec)
			assertMountCall(t, mounter, mountCall{source: "/dev/foo", target: stagingTargetPath, fsType: tc.fsType, options: []string{"defaults"}})
		})
	}
}

func TestNodeStageVolume_FormatsUnformattedDisk(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fsType   string
		mkfsCmd  string
		mkfsArgs []string
	}{
		{name: "ext4", fsType: "ext4", mkfsCmd: "mkfs.ext4", mkfsArgs: []string{"-F", "-m0", "/dev/foo"}},
		{name: "xfs", fsType: "xfs", mkfsCmd: "mkfs.xfs", mkfsArgs: []string{"-f", "/dev/foo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stagingTargetPath := t.TempDir()
			fakeExec := newFakeExec(
				execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, err: testingexec.FakeExitError{Status: 2}},
				execCall{cmd: tc.mkfsCmd, args: tc.mkfsArgs},
			)
			mounter := &recordingMounter{}
			n := newTestNode(mounter, fakeExec)

			_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, tc.fsType))
			if err != nil {
				t.Fatalf("NodeStageVolume() error = %v", err)
			}
			assertExecConsumed(t, fakeExec)
			assertMountCall(t, mounter, mountCall{source: "/dev/foo", target: stagingTargetPath, fsType: tc.fsType, options: []string{"defaults"}})
		})
	}
}

func TestNodeStageVolume_EmptyFsTypeDefaultsToExt4(t *testing.T) {
	stagingTargetPath := t.TempDir()
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, err: testingexec.FakeExitError{Status: 2}},
		execCall{cmd: "mkfs.ext4", args: []string{"-F", "-m0", "/dev/foo"}},
	)
	mounter := &recordingMounter{}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, ""))
	if err != nil {
		t.Fatalf("NodeStageVolume() error = %v", err)
	}
	assertExecConsumed(t, fakeExec)
	assertMountCall(t, mounter, mountCall{source: "/dev/foo", target: stagingTargetPath, fsType: "ext4", options: []string{"defaults"}})
}

func TestNodeStageVolume_PartitionTableIsNotFormatted(t *testing.T) {
	stagingTargetPath := t.TempDir()
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("PTTYPE=gpt\n")},
		execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
	)
	mounter := &recordingMounter{mountErr: errors.New("mount failed")}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, "ext4"))
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition", got, err)
	}
	assertExecConsumed(t, fakeExec)
	assertNoMkfs(t, fakeExec)
}

func TestNodeStageVolume_FilesystemMismatchDoesNotFormat(t *testing.T) {
	stagingTargetPath := t.TempDir()
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=xfs\n")},
		execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
	)
	mounter := &recordingMounter{mountErr: errors.New("mount failed")}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, "ext4"))
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition", got, err)
	}
	assertExecConsumed(t, fakeExec)
	assertNoMkfs(t, fakeExec)
	assertMountCall(t, mounter, mountCall{source: "/dev/foo", target: stagingTargetPath, fsType: "ext4", options: []string{"defaults"}})
}

func TestNodeStageVolume_IdempotentWhenAlreadyMounted(t *testing.T) {
	stagingTargetPath := t.TempDir()
	fakeExec := newFakeExec()
	mounter := &recordingMounter{mounts: []mount.MountPoint{{Device: "/dev/foo", Path: stagingTargetPath, Type: "ext4"}}}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", stagingTargetPath, "ext4"))
	if err != nil {
		t.Fatalf("NodeStageVolume() error = %v", err)
	}
	assertExecConsumed(t, fakeExec)
	if len(mounter.mountCalls) != 0 {
		t.Fatalf("mount calls = %v, want none", mounter.mountCalls)
	}
}

func TestNodeStageVolume_DoesNotLogRequestOrMountFlags(t *testing.T) {
	stagingTargetPath := t.TempDir()
	secretValue := "topsecret"
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=ext4\n")},
		execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
	)
	mounter := &recordingMounter{}
	n := newTestNode(mounter, fakeExec)
	req := stageRequest("vol", stagingTargetPath, "ext4")
	req.Secrets = map[string]string{"password": secretValue}
	req.VolumeContext = map[string]string{"token": secretValue}
	req.VolumeCapability.GetMount().MountFlags = []string{"password=" + secretValue}

	logs := captureKlogOutput(t, func() {
		_, err := n.NodeStageVolume(context.Background(), req)
		if err != nil {
			t.Fatalf("NodeStageVolume() error = %v", err)
		}
	})

	assertExecConsumed(t, fakeExec)
	for _, forbidden := range []string{secretValue, "mount_flags", "secrets", "volume_context", "password=" + secretValue, "token:" + secretValue} {
		if strings.Contains(logs, forbidden) {
			t.Fatalf("logs contain %q: %s", forbidden, logs)
		}
	}
}

func TestNodeStageVolume_MountFlagsAreSensitiveOptions(t *testing.T) {
	stagingTargetPath := t.TempDir()
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=ext4\n")},
	)
	mounter := &recordingMounter{}
	n := newTestNode(mounter, fakeExec)
	req := stageRequest("vol", stagingTargetPath, "ext4")
	req.VolumeCapability.GetMount().MountFlags = []string{"password=topsecret", "ro"}

	_, err := n.NodeStageVolume(context.Background(), req)
	if err != nil {
		t.Fatalf("NodeStageVolume() error = %v", err)
	}

	assertExecConsumed(t, fakeExec)
	assertMountCall(t, mounter, mountCall{
		source:           "/dev/foo",
		target:           stagingTargetPath,
		fsType:           "ext4",
		options:          []string{"defaults"},
		sensitiveOptions: []string{"password=topsecret", "ro"},
	})
}

func TestNodeStageVolume_Concurrent(t *testing.T) {
	n := &NodeService{volumeLocks: newVolumeLocks()}
	key := volumeLockKey("vol", "stage")
	if !n.volumeLocks.tryAcquire(key) {
		t.Fatal("cannot pre-acquire lock")
	}

	_, err := n.NodeStageVolume(context.Background(), stageRequest("vol", t.TempDir(), "ext4"))
	if got := status.Code(err); got != codes.Aborted {
		t.Fatalf("code = %v (err %v), want Aborted", got, err)
	}
}

func TestNodeStageVolume_BlockNoop(t *testing.T) {
	fakeExec := newFakeExec()
	mounter := &recordingMounter{}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
		VolumeId: "vol",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
		},
	})
	if err != nil {
		t.Fatalf("NodeStageVolume() error = %v", err)
	}
	assertExecConsumed(t, fakeExec)
	if len(mounter.mountCalls) != 0 {
		t.Fatalf("mount calls = %v, want none", mounter.mountCalls)
	}
}

func TestNodePublishVolume_BindMountsStagedFilesystem(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readonly bool
		options  []string
	}{
		{name: "rw", options: []string{"bind"}},
		{name: "ro", readonly: true, options: []string{"bind", "ro"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stagingTargetPath := t.TempDir()
			targetPath := t.TempDir() + "/target"
			mounter := &recordingMounter{mounts: []mount.MountPoint{{Device: "/dev/foo", Path: stagingTargetPath, Type: "ext4"}}}
			n := newTestNode(mounter, newFakeExec())
			n.getDevicePathFunc = func(context.Context, string) (string, error) {
				return "", errors.New("filesystem publish must not resolve device")
			}

			_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
				VolumeId:          "vol",
				StagingTargetPath: stagingTargetPath,
				TargetPath:        targetPath,
				Readonly:          tc.readonly,
				VolumeCapability: &csi.VolumeCapability{
					AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
				},
			})
			if err != nil {
				t.Fatalf("NodePublishVolume() error = %v", err)
			}
			assertMountCall(t, mounter, mountCall{source: stagingTargetPath, target: targetPath, options: tc.options})
		})
	}
}

func TestNodePublishVolume_StagesFilesystemWhenStagingTargetIsUnmounted(t *testing.T) {
	stagingTargetPath := t.TempDir()
	targetPath := t.TempDir() + "/target"
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=ext4\n")},
		execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
	)
	mounter := &recordingMounter{}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:          "vol",
		StagingTargetPath: stagingTargetPath,
		TargetPath:        targetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
		},
	})
	if err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}
	assertExecConsumed(t, fakeExec)
	assertMountCalls(t, mounter, []mountCall{
		{source: "/dev/foo", target: stagingTargetPath, fsType: "ext4", options: []string{"defaults"}},
		{source: stagingTargetPath, target: targetPath, options: []string{"bind"}},
	})
}

func TestNodePublishVolume_DoesNotBindUnmountedStagingTargetWhenStageFails(t *testing.T) {
	stagingTargetPath := t.TempDir()
	targetPath := t.TempDir() + "/target"
	fakeExec := newFakeExec(
		execCall{cmd: "blkid", args: []string{"-p", "-s", "TYPE", "-s", "PTTYPE", "-o", "export", "/dev/foo"}, output: []byte("TYPE=ext4\n")},
		execCall{cmd: "fsck", args: []string{"-a", "/dev/foo"}},
	)
	mounter := &recordingMounter{mountErr: errors.New("stage mount failed")}
	n := newTestNode(mounter, fakeExec)

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:          "vol",
		StagingTargetPath: stagingTargetPath,
		TargetPath:        targetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
		},
	})
	if status.Code(err) == codes.OK {
		t.Fatalf("NodePublishVolume() error = %v, want non-OK", err)
	}
	assertExecConsumed(t, fakeExec)
	for _, call := range mounter.mountCalls {
		if call.source == stagingTargetPath && call.target == targetPath {
			t.Fatalf("bind mount from unmounted staging target was called: %v", mounter.mountCalls)
		}
	}
}

func TestNodePublishVolume_FilesystemAlreadyPublishedDoesNotRequireStaging(t *testing.T) {
	stagingTargetPath := t.TempDir()
	targetPath := t.TempDir() + "/target"
	mounter := &recordingMounter{mounts: []mount.MountPoint{{Device: stagingTargetPath, Path: targetPath}}}
	fakeExec := newFakeExec()
	n := newTestNode(mounter, fakeExec)
	n.getDevicePathFunc = func(context.Context, string) (string, error) {
		return "", errors.New("already-published filesystem must not resolve device")
	}

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:          "vol",
		StagingTargetPath: stagingTargetPath,
		TargetPath:        targetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
		},
	})
	if err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}
	assertExecConsumed(t, fakeExec)
	if len(mounter.mountCalls) != 0 {
		t.Fatalf("mount calls = %v, want none", mounter.mountCalls)
	}
}

func TestNodePublishVolume_MissingStagingTarget(t *testing.T) {
	n := newTestNode(&recordingMounter{}, newFakeExec())

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:   "vol",
		TargetPath: t.TempDir() + "/target",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
		},
	})
	if got := status.Code(err); got != codes.FailedPrecondition {
		t.Fatalf("code = %v (err %v), want FailedPrecondition", got, err)
	}
}

func TestNodeUnstageVolume_Cleanup(t *testing.T) {
	t.Run("mounted", func(t *testing.T) {
		stagingTargetPath := t.TempDir()
		mounter := &recordingMounter{mounts: []mount.MountPoint{{Device: "/dev/foo", Path: stagingTargetPath, Type: "ext4"}}}
		n := newTestNode(mounter, newFakeExec())

		_, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{VolumeId: "vol", StagingTargetPath: stagingTargetPath})
		if err != nil {
			t.Fatalf("NodeUnstageVolume() error = %v", err)
		}
		if !reflect.DeepEqual(mounter.unmountCalls, []string{stagingTargetPath}) {
			t.Fatalf("unmount calls = %v, want %v", mounter.unmountCalls, []string{stagingTargetPath})
		}
		if _, err := os.Stat(stagingTargetPath); !os.IsNotExist(err) {
			t.Fatalf("staging path must be removed, stat err = %v", err)
		}
	})

	t.Run("absent", func(t *testing.T) {
		stagingTargetPath := t.TempDir() + "/missing"
		n := newTestNode(&recordingMounter{}, newFakeExec())

		_, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{VolumeId: "vol", StagingTargetPath: stagingTargetPath})
		if err != nil {
			t.Fatalf("NodeUnstageVolume() error = %v", err)
		}
	})

	t.Run("already unmounted", func(t *testing.T) {
		stagingTargetPath := t.TempDir()
		mounter := &recordingMounter{}
		n := newTestNode(mounter, newFakeExec())

		_, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{VolumeId: "vol", StagingTargetPath: stagingTargetPath})
		if err != nil {
			t.Fatalf("NodeUnstageVolume() error = %v", err)
		}
		if len(mounter.unmountCalls) != 0 {
			t.Fatalf("unmount calls = %v, want none", mounter.unmountCalls)
		}
	})
}

func TestNodeUnpublishVolume_UsesExtensiveCleanupForBindTarget(t *testing.T) {
	targetPath := t.TempDir() + "/target"
	if err := os.Mkdir(targetPath, 0o750); err != nil {
		t.Fatalf("failed to create target path: %v", err)
	}
	mounter := &recordingMounter{
		mounts:               []mount.MountPoint{{Device: "/stage", Path: targetPath}},
		likelyNotMountPoints: map[string]bool{targetPath: true},
	}
	n := newTestNode(mounter, newFakeExec())

	_, err := n.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{VolumeId: "vol", TargetPath: targetPath})
	if err != nil {
		t.Fatalf("NodeUnpublishVolume() error = %v", err)
	}
	if !reflect.DeepEqual(mounter.unmountCalls, []string{targetPath}) {
		t.Fatalf("unmount calls = %v, want %v", mounter.unmountCalls, []string{targetPath})
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target path must be removed, stat err = %v", err)
	}
}

func TestNodePublishVolume_BlockBehavior(t *testing.T) {
	targetPath := t.TempDir() + "/block"
	mounter := &recordingMounter{}
	n := newTestNode(mounter, newFakeExec())

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:   "vol",
		TargetPath: targetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
		},
	})
	if err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}
	assertMountCall(t, mounter, mountCall{source: "/dev/foo", target: targetPath, options: []string{"bind"}})
}

func TestNodePublishVolume_BlockAlreadyMounted(t *testing.T) {
	targetPath := t.TempDir() + "/block"
	mounter := &recordingMounter{mounts: []mount.MountPoint{{Device: "/dev/foo", Path: targetPath}}}
	n := newTestNode(mounter, newFakeExec())

	_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
		VolumeId:   "vol",
		TargetPath: targetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
		},
	})
	if err != nil {
		t.Fatalf("NodePublishVolume() error = %v", err)
	}
	if len(mounter.mountCalls) != 0 {
		t.Fatalf("mount calls = %v, want none", mounter.mountCalls)
	}
}

func newTestNode(mounter mount.Interface, executor utilexec.Interface) *NodeService {
	return &NodeService{
		volumeLocks: newVolumeLocks(),
		mounter:     mounter,
		exec:        executor,
		getDevicePathFunc: func(context.Context, string) (string, error) {
			return "/dev/foo", nil
		},
	}
}

func stageRequest(volumeID, stagingTargetPath, fsType string) *csi.NodeStageVolumeRequest {
	return &csi.NodeStageVolumeRequest{
		VolumeId:          volumeID,
		StagingTargetPath: stagingTargetPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: fsType}},
		},
	}
}

func newFakeExec(calls ...execCall) *recordedExec {
	fakeExec := &recordedExec{FakeExec: &testingexec.FakeExec{ExactOrder: true, LookPathFunc: func(file string) (string, error) { return file, nil }}}
	for _, call := range calls {
		call := call
		fakeExec.wantCalls = append(fakeExec.wantCalls, append([]string{call.cmd}, call.args...))
		fakeExec.CommandScript = append(fakeExec.CommandScript, func(cmd string, args ...string) utilexec.Cmd {
			fakeCmd := &testingexec.FakeCmd{
				CombinedOutputScript: []testingexec.FakeAction{func() ([]byte, []byte, error) {
					return call.output, nil, call.err
				}},
			}
			return testingexec.InitFakeCmd(fakeCmd, call.cmd, call.args...)
		})
	}
	return fakeExec
}

func captureKlogOutput(t *testing.T, fn func()) string {
	t.Helper()
	state := klog.CaptureState()
	defer state.Restore()
	buf := new(bytes.Buffer)
	klog.SetOutput(buf)
	klog.LogToStderr(false)

	fn()
	klog.Flush()

	return buf.String()
}

func assertExecConsumed(t *testing.T, fakeExec *recordedExec) {
	t.Helper()
	if fakeExec.CommandCalls != len(fakeExec.CommandScript) {
		t.Fatalf("exec calls = %d, want %d", fakeExec.CommandCalls, len(fakeExec.CommandScript))
	}
}

func commandCalls(fakeExec *recordedExec) [][]string {
	return fakeExec.wantCalls[:fakeExec.CommandCalls]
}

func assertNoMkfs(t *testing.T, fakeExec *recordedExec) {
	t.Helper()
	for _, call := range commandCalls(fakeExec) {
		if len(call) > 0 && len(call[0]) >= len("mkfs") && call[0][:len("mkfs")] == "mkfs" {
			t.Fatalf("mkfs command must not be called, got %v", commandCalls(fakeExec))
		}
	}
}

func assertMountCall(t *testing.T, mounter *recordingMounter, want mountCall) {
	t.Helper()
	if len(mounter.mountCalls) != 1 {
		t.Fatalf("mount calls = %v, want one call", mounter.mountCalls)
	}
	got := mounter.mountCalls[0]
	if got.source != want.source || got.target != want.target || got.fsType != want.fsType || !reflect.DeepEqual(got.options, want.options) || !reflect.DeepEqual(got.sensitiveOptions, want.sensitiveOptions) {
		t.Fatalf("mount call = %+v, want %+v", got, want)
	}
}

func assertMountCalls(t *testing.T, mounter *recordingMounter, want []mountCall) {
	t.Helper()
	if !reflect.DeepEqual(mounter.mountCalls, want) {
		t.Fatalf("mount calls = %+v, want %+v", mounter.mountCalls, want)
	}
}
