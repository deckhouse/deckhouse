//go:build !integration

/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package waypointcontroller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func TestResourcesFromResourcesManagement_Dispatcher(t *testing.T) {
	cases := []struct {
		name       string
		spec       *networkv1alpha1.ResourcesManagement
		wantErr    bool
		wantReqCPU string
		wantReqMem string
	}{
		{name: "nil_spec", spec: nil, wantErr: false, wantReqCPU: "100m", wantReqMem: "128Mi"},
		{name: "mode_VPA", spec: &networkv1alpha1.ResourcesManagement{Mode: "VPA"}, wantErr: false, wantReqCPU: "100m", wantReqMem: "128Mi"},
		{name: "mode_empty", spec: &networkv1alpha1.ResourcesManagement{Mode: ""}, wantErr: false, wantReqCPU: "100m", wantReqMem: "128Mi"},
		{name: "mode_Static", spec: &networkv1alpha1.ResourcesManagement{Mode: "Static"}, wantErr: false, wantReqCPU: "100m", wantReqMem: "128Mi"},
		{name: "unknown_mode", spec: &networkv1alpha1.ResourcesManagement{Mode: "garbage"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqs, err := resourcesFromResourcesManagement(tc.spec)
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tc.wantReqCPU != "" {
				assertQ(t, reqs.Requests, corev1.ResourceCPU, tc.wantReqCPU)
			}
			if tc.wantReqMem != "" {
				assertQ(t, reqs.Requests, corev1.ResourceMemory, tc.wantReqMem)
			}
		})
	}
}

func TestResourcesFromResourcesManagementStatic(t *testing.T) {
	cases := []struct {
		name       string
		requests   *networkv1alpha1.ResourcesRequestsLimits
		limits     *networkv1alpha1.ResourcesRequestsLimits
		wantReqCPU string
		wantReqMem string
		wantLimCPU string
		wantLimMem string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "all_defaults",
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
		},
		{
			name:       "custom_requests",
			requests:   &networkv1alpha1.ResourcesRequestsLimits{CPU: "250m", Memory: "1Gi"},
			wantReqCPU: "250m",
			wantReqMem: "1Gi",
		},
		{
			name:       "requests_and_limits",
			requests:   &networkv1alpha1.ResourcesRequestsLimits{CPU: "200m", Memory: "256Mi"},
			limits:     &networkv1alpha1.ResourcesRequestsLimits{CPU: "500m", Memory: "512Mi"},
			wantReqCPU: "200m",
			wantReqMem: "256Mi",
			wantLimCPU: "500m",
			wantLimMem: "512Mi",
		},
		{
			name:       "only_limits_no_requests",
			limits:     &networkv1alpha1.ResourcesRequestsLimits{CPU: "500m", Memory: "1Gi"},
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
			wantLimCPU: "500m",
			wantLimMem: "1Gi",
		},
		{
			name:       "partial_request_override",
			requests:   &networkv1alpha1.ResourcesRequestsLimits{CPU: "100m"},
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
		},
		{
			name:     "bad_cpu_request",
			requests: &networkv1alpha1.ResourcesRequestsLimits{CPU: "xyz"},
			wantErr:  true,
			errMsg:   "resourcesManagement.static.requests.cpu",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := &networkv1alpha1.ResourcesManagement{
				Mode: "Static",
				Static: &networkv1alpha1.ResourcesStatic{
					Requests: tc.requests,
					Limits:   tc.limits,
				},
			}

			reqs, err := resourcesFromResourcesManagementStatic(spec)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q missing substring %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertQ(t, reqs.Requests, corev1.ResourceCPU, tc.wantReqCPU)
			assertQ(t, reqs.Requests, corev1.ResourceMemory, tc.wantReqMem)

			assertQ(t, reqs.Limits, corev1.ResourceCPU, tc.wantLimCPU)
			assertQ(t, reqs.Limits, corev1.ResourceMemory, tc.wantLimMem)

			if _, ok := reqs.Requests[corev1.ResourceEphemeralStorage]; !ok {
				t.Errorf("missing ephemeral-storage request")
			}
		})
	}
}

func TestResourcesFromResourcesManagementStatic_NilAll(t *testing.T) {
	reqs, err := resourcesFromResourcesManagementStatic(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqs.Limits != nil {
		t.Errorf("expected nil Limits for nil-static, got %v", reqs.Limits)
	}
	assertQ(t, reqs.Requests, corev1.ResourceCPU, "100m")
	assertQ(t, reqs.Requests, corev1.ResourceMemory, "128Mi")
	if _, ok := reqs.Requests[corev1.ResourceEphemeralStorage]; !ok {
		t.Errorf("missing ephemeral-storage request")
	}
}

func TestResourcesFromResourcesManagementStatic_StaticNil(t *testing.T) {
	spec := &networkv1alpha1.ResourcesManagement{Mode: "Static"}
	reqs, err := resourcesFromResourcesManagementStatic(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqs.Limits != nil {
		t.Errorf("expected nil Limits when static is nil, got %v", reqs.Limits)
	}
	assertQ(t, reqs.Requests, corev1.ResourceCPU, "100m")
	assertQ(t, reqs.Requests, corev1.ResourceMemory, "128Mi")
	if _, ok := reqs.Requests[corev1.ResourceEphemeralStorage]; !ok {
		t.Errorf("missing ephemeral-storage request")
	}
}

func TestResourcesFromResourcesManagementVPA(t *testing.T) {
	cases := []struct {
		name       string
		spec       *networkv1alpha1.ResourcesManagement
		wantReqCPU string
		wantReqMem string
		wantLimCPU string
		wantLimMem string
	}{
		{
			name:       "defaults_no_spec",
			spec:       nil,
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
		},
		{
			name:       "custom_min_cpu_and_mem",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{CPU: &networkv1alpha1.VPAResource{Min: "200m"}, Memory: &networkv1alpha1.VPAResource{Min: "1Gi"}}},
			wantReqCPU: "200m",
			wantReqMem: "1Gi",
		},
		{
			name:       "cpu_limitRatio_2",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{CPU: &networkv1alpha1.VPAResource{Min: "100m", LimitRatio: limitRatioPtr(2.0)}}},
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
			wantLimCPU: "200m",
		},
		{
			name:       "memory_limitRatio_1_5",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{Memory: &networkv1alpha1.VPAResource{Min: "500Mi", LimitRatio: limitRatioPtr(1.5)}}},
			wantReqCPU: "100m",
			wantReqMem: "500Mi",
			wantLimMem: "750Mi",
		},
		{
			name:       "limitRatio_zero_ignored",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{CPU: &networkv1alpha1.VPAResource{Min: "100m", LimitRatio: limitRatioPtr(0)}}},
			wantReqCPU: "100m",
			wantReqMem: "128Mi",
		},
		{
			name:       "limitRatio_negative_ignored",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{Memory: &networkv1alpha1.VPAResource{Min: "100Mi", LimitRatio: limitRatioPtr(-1.0)}}},
			wantReqCPU: "100m",
			wantReqMem: "100Mi",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqs, err := resourcesFromResourcesManagementVPA(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertQ(t, reqs.Requests, corev1.ResourceCPU, tc.wantReqCPU)
			assertQ(t, reqs.Requests, corev1.ResourceMemory, tc.wantReqMem)
			assertQ(t, reqs.Limits, corev1.ResourceCPU, tc.wantLimCPU)
			assertQ(t, reqs.Limits, corev1.ResourceMemory, tc.wantLimMem)

			if _, ok := reqs.Requests[corev1.ResourceEphemeralStorage]; !ok {
				t.Errorf("missing ephemeral-storage request")
			}
		})
	}
}

func TestVPAContainerResourcePolicies(t *testing.T) {
	cases := []struct {
		name       string
		spec       *networkv1alpha1.ResourcesManagement
		wantMinCPU string
		wantMaxCPU string
		wantMinMem string
		wantMaxMem string
	}{
		{
			name:       "all_defaults_nil_spec",
			spec:       nil,
			wantMinCPU: "100m",
			wantMaxCPU: "1000m",
			wantMinMem: "128Mi",
			wantMaxMem: "2000Mi",
		},
		{
			name:       "custom_min_max",
			spec:       &networkv1alpha1.ResourcesManagement{Mode: "VPA", VPA: &networkv1alpha1.ResourcesVPA{CPU: &networkv1alpha1.VPAResource{Min: "200m", Max: "1"}, Memory: &networkv1alpha1.VPAResource{Min: "1Gi", Max: "4Gi"}}},
			wantMinCPU: "200m",
			wantMaxCPU: "1",
			wantMinMem: "1Gi",
			wantMaxMem: "4Gi",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policies, err := vpaContainerResourcePolicies(tc.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(policies) != 1 {
				t.Fatalf("expected 1 policy, got %d", len(policies))
			}
			p := policies[0]

			if p.ContainerName != waypointProxyContainerName {
				t.Errorf("container name = %q, want %q", p.ContainerName, waypointProxyContainerName)
			}
			if p.ControlledValues == nil || *p.ControlledValues != vpav1.ContainerControlledValuesRequestsAndLimits {
				t.Error("controlled values != RequestsAndLimits")
			}

			assertQ(t, p.MinAllowed, corev1.ResourceCPU, tc.wantMinCPU)
			assertQ(t, p.MaxAllowed, corev1.ResourceCPU, tc.wantMaxCPU)
			assertQ(t, p.MinAllowed, corev1.ResourceMemory, tc.wantMinMem)
			assertQ(t, p.MaxAllowed, corev1.ResourceMemory, tc.wantMaxMem)
		})
	}
}

func TestVPAUpdateMode(t *testing.T) {
	cases := []struct {
		name       string
		spec       *networkv1alpha1.WaypointInstance
		wantUpdate vpav1.UpdateMode
	}{
		{
			name:       "nil_resourcesManagement",
			spec:       newInstance("main", "ns"),
			wantUpdate: vpav1.UpdateModeInPlaceOrRecreate,
		},
		{
			name:       "nil_VPA",
			spec:       newInstance("main", "ns", withResourcesManagementMode("VPA")),
			wantUpdate: vpav1.UpdateModeInPlaceOrRecreate,
		},
		{
			name:       "empty_mode",
			spec:       newInstance("main", "ns", withVPAMode("")),
			wantUpdate: vpav1.UpdateModeInPlaceOrRecreate,
		},
		{
			name:       "mode_Initial",
			spec:       newInstance("main", "ns", withVPAMode("Initial")),
			wantUpdate: vpav1.UpdateModeInitial,
		},
		{
			name:       "mode_InPlaceOrRecreate",
			spec:       newInstance("main", "ns", withVPAMode("InPlaceOrRecreate")),
			wantUpdate: vpav1.UpdateModeInPlaceOrRecreate,
		},
		{
			name:       "unknown_mode_defaults_to_InPlaceOrRecreate",
			spec:       newInstance("main", "ns", withVPAMode("garbage")),
			wantUpdate: vpav1.UpdateModeInPlaceOrRecreate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := vpaUpdateMode(tc.spec.Spec.ResourcesManagement)
			if got == nil {
				t.Fatal("expected non-nil update mode")
			}
			if *got != tc.wantUpdate {
				t.Errorf("update mode = %q, want %q", *got, tc.wantUpdate)
			}
		})
	}
}

func TestNewVPAForWaypoint(t *testing.T) {
	inst := newInstance("main", "ns")
	vpa, err := newVPAForWaypoint(inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vpa.Name != resourceBaseName(inst.Name) {
		t.Errorf("name = %q, want %q", vpa.Name, resourceBaseName(inst.Name))
	}
	if vpa.Namespace != inst.Namespace {
		t.Errorf("namespace = %q, want %q", vpa.Namespace, inst.Namespace)
	}
	if vpa.Spec.TargetRef.Kind != "Deployment" {
		t.Errorf("targetRef kind = %q, want Deployment", vpa.Spec.TargetRef.Kind)
	}
	if vpa.Spec.TargetRef.Name != resourceBaseName(inst.Name) {
		t.Errorf("targetRef name = %q, want %q", vpa.Spec.TargetRef.Name, resourceBaseName(inst.Name))
	}
	if vpa.Spec.UpdatePolicy == nil || vpa.Spec.UpdatePolicy.UpdateMode == nil || *vpa.Spec.UpdatePolicy.UpdateMode != vpav1.UpdateModeInPlaceOrRecreate {
		t.Error("missing or incorrect update mode")
	}
	if vpa.Labels[WaypointComponentLabelKey] != "vpa" {
		t.Errorf("component label = %q, want vpa", vpa.Labels[WaypointComponentLabelKey])
	}
}

func TestQuantityFromRatio(t *testing.T) {
	verifyRatio := func(rd *resource.Quantity, ratio float64, resName corev1.ResourceName, want *resource.Quantity, desc string) {
		t.Helper()
		got := quantityFromRatio(*rd, ratio, resName)
		if got.Cmp(*want) != 0 {
			t.Errorf("%s: quantityFromRatio(%v, %v, %s): want CanonicalValue=%d, got=%d", desc, rd, ratio, resName, (*want).AsDec(), got.AsDec())
		}
	}

	base100m := mustParseQ(t, "100m")
	base333m := mustParseQ(t, "333m")
	base500Mi := mustParseQ(t, "500Mi")
	base100Mi := mustParseQ(t, "100Mi")

	// Use resource.NewQuantity for the expected values to avoid parse ambiguity.
	want200m := resource.NewMilliQuantity(200, resource.DecimalSI)
	want101m := resource.NewMilliQuantity(101, resource.DecimalSI)
	want500m := resource.NewMilliQuantity(500, resource.DecimalSI)
	want1000Mi := resource.NewQuantity(500*1024*1024*2, resource.BinarySI)
	// 100Mi * 1.1 = 104857600 * 1.1 = 115343360.0, but float64(1.1) adds tiny imprecision
	// so ceil(float64(104857600) * 1.1) = 115343361
	want110MiCeil := resource.NewQuantity(115343361, resource.BinarySI)
	wantZero := resource.Quantity{}

	verifyRatio(&base100m, 2.0, corev1.ResourceCPU, want200m, "cpu_no_fraction")
	verifyRatio(&base100m, 1.005, corev1.ResourceCPU, want101m, "cpu_ceil")
	verifyRatio(&base333m, 1.5, corev1.ResourceCPU, want500m, "cpu_round_up")
	verifyRatio(&base500Mi, 2.0, corev1.ResourceMemory, want1000Mi, "memory_exact")
	verifyRatio(&base100Mi, 1.1, corev1.ResourceMemory, want110MiCeil, "memory_ceil")
	verifyRatio(&base100m, 2.0, corev1.ResourceName("bogus"), &wantZero, "unknown_resource")
}

func TestParseOptionalQuantity(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		path    string
		wantNil bool
		wantErr bool
		errMsg  string
		wantStr string
	}{
		{name: "empty", value: "", path: "field.x", wantNil: true},
		{name: "whitespace", value: "  ", path: "field.x", wantNil: true},
		{name: "valid", value: "100m", path: "field.x", wantStr: "100m"},
		{name: "invalid", value: "xyz", path: "resourcesManagement.vpa.cpu.min", wantErr: true, errMsg: "resourcesManagement.vpa.cpu.min"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q, err := parseOptionalQuantity(tc.value, tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errMsg != "" && !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("error %q missing substring %q", err.Error(), tc.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if q != nil {
					t.Errorf("expected nil quantity, got %v", q)
				}
				return
			}
			if q == nil {
				t.Fatal("expected non-nil quantity")
			}
			wantQ := mustParseQ(t, tc.wantStr)
			if q.Cmp(wantQ) != 0 {
				t.Errorf("quantity.Cmp(%s) != 0, got=%v, want=%v", tc.wantStr, q, wantQ)
			}
		})
	}
}

func TestSetDefaultEphemeralStorageRequest(t *testing.T) {
	t.Run("nil_reqs", func(t *testing.T) {
		if err := setDefaultEphemeralStorageRequest(nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("sets_default", func(t *testing.T) {
		reqs := &corev1.ResourceRequirements{}
		if err := setDefaultEphemeralStorageRequest(reqs); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertQ(t, reqs.Requests, corev1.ResourceEphemeralStorage, defaultProxyEphemeralStorageRequest)
	})
}

func TestFinalizeResourceRequirements_RemovesEmptyLimits(t *testing.T) {
	reqs := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
	if err := finalizeResourceRequirements(reqs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqs.Limits != nil {
		t.Error("expected nil Limits after finalize")
	}
}

func TestResourceConstants(t *testing.T) {
	if waypointProxyContainerName != "istio-proxy" {
		t.Errorf("waypointProxyContainerName = %q, want istio-proxy", waypointProxyContainerName)
	}
	if defaultVPACPUMin != "100m" {
		t.Errorf("defaultVPACPUMin = %q, want 100m", defaultVPACPUMin)
	}
	if defaultVPACPUMax != "1000m" {
		t.Errorf("defaultVPACPUMax = %q, want 1000m", defaultVPACPUMax)
	}
	if defaultVPAMemoryMin != "128Mi" {
		t.Errorf("defaultVPAMemoryMin = %q, want 128Mi", defaultVPAMemoryMin)
	}
	if defaultVPAMemoryMax != "2000Mi" {
		t.Errorf("defaultVPAMemoryMax = %q, want 2000Mi", defaultVPAMemoryMax)
	}
	if defaultStaticCPURequest != "100m" {
		t.Errorf("defaultStaticCPURequest = %q, want 100m", defaultStaticCPURequest)
	}
	if defaultStaticMemoryRequest != "128Mi" {
		t.Errorf("defaultStaticMemoryRequest = %q, want 128Mi", defaultStaticMemoryRequest)
	}
	if defaultProxyEphemeralStorageRequest != "50Mi" {
		t.Errorf("defaultProxyEphemeralStorageRequest = %q, want 50Mi", defaultProxyEphemeralStorageRequest)
	}
}

func assertQ(t *testing.T, rl corev1.ResourceList, name corev1.ResourceName, want string) {
	t.Helper()
	q, ok := rl[name]
	if want == "" {
		if ok {
			t.Errorf("unexpected %s present", name)
		}
		return
	}
	if !ok {
		t.Errorf("missing %s in resource list (want %s)", name, want)
		return
	}
	wantQ := mustParseQ(t, want)
	if q.Cmp(wantQ) != 0 {
		t.Errorf("%s mismatch (want %s)", name, want)
	}
}
