/*
Copyright 2025 Flant JSC

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

package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	"github.com/deckhouse/virtualization/api/core/v1alpha2/vdcondition"
)

const testNamespace = "default"

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1alpha2.AddToScheme(s); err != nil {
		t.Fatalf("v1alpha2.AddToScheme: %v", err)
	}
	return s
}

func newDiskServiceWithDisks(t *testing.T, disks ...*v1alpha2.VirtualDisk) *DiskService {
	t.Helper()
	objs := make([]ctrlclient.Object, len(disks))
	for i, d := range disks {
		objs[i] = d
	}
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objs...).
		Build()
	return &DiskService{&Service{client: c, namespace: testNamespace}}
}

func makeVMD(name string, phase v1alpha2.DiskPhase, conditions []metav1.Condition) *v1alpha2.VirtualDisk {
	return &v1alpha2.VirtualDisk{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{diskNameLabel: name},
		},
		Status: v1alpha2.VirtualDiskStatus{
			Phase:      phase,
			Conditions: conditions,
		},
	}
}

func readyCond(reason vdcondition.ReadyReason, status metav1.ConditionStatus, msg string) metav1.Condition {
	return metav1.Condition{
		Type:    vdcondition.ReadyType.String(),
		Status:  status,
		Reason:  reason.String(),
		Message: msg,
	}
}

func TestTerminalDiskError(t *testing.T) {
	tests := []struct {
		name      string
		vmd       *v1alpha2.VirtualDisk
		wantErr   bool
		isQuota   bool
		wantInMsg string
	}{
		{
			name:    "provisioning phase, no conditions — no error",
			vmd:     makeVMD("d", v1alpha2.DiskProvisioning, nil),
			wantErr: false,
		},
		{
			name: "Ready condition True — no error",
			vmd: makeVMD("d", v1alpha2.DiskReady, []metav1.Condition{
				readyCond(vdcondition.Ready, metav1.ConditionTrue, ""),
			}),
			wantErr: false,
		},
		{
			name: "non-terminal ReadyReason (Provisioning) with ConditionFalse — no error",
			vmd: makeVMD("d", v1alpha2.DiskProvisioning, []metav1.Condition{
				readyCond(vdcondition.Provisioning, metav1.ConditionFalse, "in progress"),
			}),
			wantErr: false,
		},
		{
			name: "QuotaExceeded wraps ErrQuotaExceeded",
			vmd: makeVMD("d", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.QuotaExceeded, metav1.ConditionFalse, "quota exceeded msg"),
			}),
			wantErr:   true,
			isQuota:   true,
			wantInMsg: "quota exceeded msg",
		},
		{
			name: "ProvisioningFailed",
			vmd: makeVMD("d", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.ProvisioningFailed, metav1.ConditionFalse, "out of space"),
			}),
			wantErr:   true,
			wantInMsg: "out of space",
		},
		{
			name: "Lost",
			vmd: makeVMD("d", v1alpha2.DiskLost, []metav1.Condition{
				readyCond(vdcondition.Lost, metav1.ConditionFalse, "pvc lost"),
			}),
			wantErr:   true,
			wantInMsg: "pvc lost",
		},
		{
			name: "ImagePullFailed",
			vmd: makeVMD("d", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.ImagePullFailed, metav1.ConditionFalse, "pull failed"),
			}),
			wantErr:   true,
			wantInMsg: "pull failed",
		},
		{
			name: "DatasourceIsNotFound",
			vmd: makeVMD("d", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.DatasourceIsNotFound, metav1.ConditionFalse, "datasource gone"),
			}),
			wantErr:   true,
			wantInMsg: "datasource gone",
		},
		{
			name: "StorageClassIsNotReady",
			vmd: makeVMD("d", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.StorageClassIsNotReady, metav1.ConditionFalse, "sc not ready"),
			}),
			wantErr:   true,
			wantInMsg: "sc not ready",
		},
		{
			name:      "DiskFailed phase without terminal condition",
			vmd:       makeVMD("d", v1alpha2.DiskFailed, nil),
			wantErr:   true,
			wantInMsg: `"d" is in Failed phase`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := TerminalDiskError(tc.vmd)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.isQuota && !errors.Is(err, ErrQuotaExceeded) {
					t.Fatalf("expected ErrQuotaExceeded, got: %v", err)
				}
				if tc.wantInMsg != "" && !strings.Contains(err.Error(), tc.wantInMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantInMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWaitDiskCreation(t *testing.T) {
	tests := []struct {
		name      string
		disk      *v1alpha2.VirtualDisk
		ctx       func() context.Context
		wantErr   bool
		isQuota   bool
		isCtxErr  bool
		wantInMsg string
	}{
		{
			name:    "DiskReady — success",
			disk:    makeVMD("disk1", v1alpha2.DiskReady, nil),
			wantErr: false,
		},
		{
			name:    "DiskWaitForFirstConsumer — success",
			disk:    makeVMD("disk1", v1alpha2.DiskWaitForFirstConsumer, nil),
			wantErr: false,
		},
		{
			name: "QuotaExceeded — returns ErrQuotaExceeded",
			disk: makeVMD("disk1", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.QuotaExceeded, metav1.ConditionFalse, "quota exceeded"),
			}),
			wantErr: true,
			isQuota: true,
		},
		{
			name: "ProvisioningFailed terminal condition",
			disk: makeVMD("disk1", v1alpha2.DiskPending, []metav1.Condition{
				readyCond(vdcondition.ProvisioningFailed, metav1.ConditionFalse, "out of space"),
			}),
			wantErr:   true,
			wantInMsg: "out of space",
		},
		{
			name:      "DiskFailed phase",
			disk:      makeVMD("disk1", v1alpha2.DiskFailed, nil),
			wantErr:   true,
			wantInMsg: `"disk1" is in Failed phase`,
		},
		{
			name: "context cancelled while waiting for provisioning disk",
			disk: makeVMD("disk1", v1alpha2.DiskProvisioning, nil),
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr:  true,
			isCtxErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newDiskServiceWithDisks(t, tc.disk)

			ctx := context.Background()
			if tc.ctx != nil {
				ctx = tc.ctx()
			}

			err := svc.WaitDiskCreation(ctx, "disk1")

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.isQuota && !errors.Is(err, ErrQuotaExceeded) {
					t.Fatalf("expected ErrQuotaExceeded, got: %v", err)
				}
				if tc.isCtxErr && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected context error, got: %v", err)
				}
				if tc.wantInMsg != "" && !strings.Contains(err.Error(), tc.wantInMsg) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantInMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
