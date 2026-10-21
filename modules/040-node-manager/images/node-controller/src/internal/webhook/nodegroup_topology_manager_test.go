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

package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func TestValidation_TopologyManagerReservation(t *testing.T) {
	cpu := func(value intstr.IntOrString) *v1.ResourceReservationSpec {
		return &v1.ResourceReservationSpec{Mode: "Static", Static: &v1.StaticResourceReservation{CPU: &value}}
	}
	reservations := []struct {
		name        string
		reservation *v1.ResourceReservationSpec
		errorPart   string
	}{
		{name: "missing", errorPart: "resourceReservation must be enabled"},
		{name: "Off", reservation: &v1.ResourceReservationSpec{Mode: "Off"}, errorPart: "mode must be Auto or Static"},
		{name: "empty mode", reservation: &v1.ResourceReservationSpec{}, errorPart: "mode must be Auto or Static"},
		{name: "Auto", reservation: &v1.ResourceReservationSpec{Mode: "Auto"}},
		{name: "Static no subtree", reservation: &v1.ResourceReservationSpec{Mode: "Static"}, errorPart: "static.cpu must be specified"},
		{name: "Static no CPU", reservation: &v1.ResourceReservationSpec{Mode: "Static", Static: &v1.StaticResourceReservation{}}, errorPart: "static.cpu must be specified"},
		{name: "70m", reservation: cpu(intstr.FromString("70m"))},
		{name: "one integer CPU", reservation: cpu(intstr.FromInt32(1))},
		{name: "zero integer CPU", reservation: cpu(intstr.FromInt32(0)), errorPart: "positive CPU quantity"},
		{name: "zero milli CPU", reservation: cpu(intstr.FromString("0m")), errorPart: "positive CPU quantity"},
		{name: "negative CPU", reservation: cpu(intstr.FromString("-500m")), errorPart: "positive CPU quantity"},
		{name: "invalid CPU", reservation: cpu(intstr.FromString("invalid")), errorPart: "positive CPU quantity"},
	}

	for _, policy := range []string{"BestEffort", "Restricted", "SingleNumaNode"} {
		for _, reservation := range reservations {
			for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
				t.Run(policy+"/"+reservation.name+"/"+string(operation), func(t *testing.T) {
					kubelet := &v1.KubeletSpec{
						TopologyManager:     &v1.TopologyManagerSpec{Policy: policy},
						ResourceReservation: reservation.reservation,
					}
					response := topologyManagerAdmission(t, operation, kubelet, kubelet)
					if reservation.errorPart == "" {
						require.True(t, response.Allowed, "%v", response.Result)
					} else {
						require.False(t, response.Allowed)
						require.Contains(t, response.Result.Message, reservation.errorPart)
					}
				})
			}
		}
	}
}

func TestValidation_TopologyManagerWithoutPinning(t *testing.T) {
	off := &v1.ResourceReservationSpec{Mode: "Off"}
	cases := []struct {
		name    string
		kubelet *v1.KubeletSpec
	}{
		{name: "nil kubelet"},
		{name: "empty kubelet", kubelet: &v1.KubeletSpec{ResourceReservation: off}},
		{name: "empty topologyManager", kubelet: &v1.KubeletSpec{TopologyManager: &v1.TopologyManagerSpec{}, ResourceReservation: off}},
		{name: "None", kubelet: &v1.KubeletSpec{TopologyManager: &v1.TopologyManagerSpec{Policy: "None"}, ResourceReservation: off}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := topologyManagerAdmission(t, admissionv1.Create, tc.kubelet, nil)
			require.True(t, response.Allowed, "%v", response.Result)
		})
	}
}

func TestValidation_TopologyManagerEnabledIgnored(t *testing.T) {
	disabled := false
	kubelet := &v1.KubeletSpec{
		TopologyManager:     &v1.TopologyManagerSpec{Enabled: &disabled, Policy: "SingleNumaNode"},
		ResourceReservation: &v1.ResourceReservationSpec{Mode: "Off"},
	}
	response := topologyManagerAdmission(t, admissionv1.Create, kubelet, nil)
	require.False(t, response.Allowed, "enabled: false must not turn the policy off")
	require.Contains(t, response.Result.Message, "mode must be Auto or Static")
}

func TestValidation_TopologyManagerWarnings(t *testing.T) {
	enabled, disabled := true, false
	withEnabled := func(k *v1.KubeletSpec, value *bool) *v1.KubeletSpec {
		k.TopologyManager.Enabled = value
		return k
	}
	configured := func(policy, scope string) *v1.KubeletSpec {
		return &v1.KubeletSpec{
			TopologyManager:     &v1.TopologyManagerSpec{Policy: policy, Scope: scope},
			ResourceReservation: &v1.ResourceReservationSpec{Mode: "Auto"},
		}
	}
	const (
		restart    = "kubelet restart"
		disruption = "disruption"
		noEffect   = "enabled is deprecated and has no effect, the resource managers"
		notOff     = "enabled: false does not turn off the policy SingleNumaNode"
	)
	cases := []struct {
		name      string
		operation admissionv1.Operation
		old, new  *v1.KubeletSpec
		warnings  []string
	}{
		{name: "CREATE SingleNumaNode", operation: admissionv1.Create, new: configured("SingleNumaNode", "")},
		{name: "CREATE enabled true", operation: admissionv1.Create, new: withEnabled(configured("SingleNumaNode", ""), &enabled), warnings: []string{noEffect}},
		{name: "CREATE enabled false with a policy", operation: admissionv1.Create, new: withEnabled(configured("SingleNumaNode", ""), &disabled), warnings: []string{notOff}},
		{name: "stored default enabled false with None", operation: admissionv1.Update, old: withEnabled(configured("None", ""), &disabled), new: withEnabled(configured("None", ""), &disabled)},
		{name: "enabled false is ignored", operation: admissionv1.Update, old: configured("SingleNumaNode", ""), new: withEnabled(configured("SingleNumaNode", ""), &disabled), warnings: []string{notOff}},
		{name: "enabled false does not hide a change", operation: admissionv1.Update, old: withEnabled(configured("None", ""), &disabled), new: withEnabled(configured("SingleNumaNode", ""), &disabled), warnings: []string{restart, disruption, notOff}},
		{name: "unchanged", operation: admissionv1.Update, old: configured("Restricted", ""), new: configured("Restricted", "")},
		{name: "default scope is Pod", operation: admissionv1.Update, old: configured("Restricted", ""), new: configured("Restricted", "Pod")},
		{name: "scope change with None", operation: admissionv1.Update, old: configured("None", "Pod"), new: configured("None", "Container")},
		{name: "None to BestEffort", operation: admissionv1.Update, old: configured("None", ""), new: configured("BestEffort", ""), warnings: []string{restart, disruption}},
		{name: "BestEffort to SingleNumaNode", operation: admissionv1.Update, old: configured("BestEffort", ""), new: configured("SingleNumaNode", ""), warnings: []string{restart, disruption}},
		{name: "scope change", operation: admissionv1.Update, old: configured("Restricted", "Pod"), new: configured("Restricted", "Container"), warnings: []string{restart, disruption}},
		{name: "to None", operation: admissionv1.Update, old: configured("Restricted", ""), new: configured("None", ""), warnings: []string{restart}},
		{name: "remove kubelet", operation: admissionv1.Update, old: configured("Restricted", ""), warnings: []string{restart}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := topologyManagerAdmission(t, tc.operation, tc.new, tc.old)
			require.True(t, response.Allowed, "%v", response.Result)
			require.Len(t, response.Warnings, len(tc.warnings), "%v", response.Warnings)
			for index, text := range tc.warnings {
				require.Contains(t, response.Warnings[index], text)
				if text == noEffect || text == notOff {
					require.Contains(t, response.Warnings[index], `d8 k patch nodegroup worker --type json -p '[{"op":"remove","path":"/spec/kubelet/topologyManager/enabled"}]'`)
				}
			}
		})
	}
}

func topologyManagerAdmission(t *testing.T, operation admissionv1.Operation, kubelet, oldKubelet *v1.KubeletSpec) admission.Response {
	t.Helper()
	scheme := newScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := &NodeGroupValidator{Client: client, decoder: admission.NewDecoder(scheme)}
	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Kubelet = kubelet
	var oldNG *v1.NodeGroup
	if operation == admissionv1.Update {
		oldNG = baseNodeGroup("worker", v1.NodeTypeStatic)
		oldNG.Spec.Kubelet = oldKubelet
	}
	return validator.Handle(context.Background(), makeAdmissionRequest(t, operation, ng, oldNG))
}
