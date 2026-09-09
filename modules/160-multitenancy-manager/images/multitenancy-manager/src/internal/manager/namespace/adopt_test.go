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

package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestTemplateFor(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "no rendered label at all",
			labels: nil,
			want:   TemplateSimple,
		},
		{
			name:   "only user labels",
			labels: map[string]string{"team": "blue"},
			want:   TemplateSimple,
		},
		{
			name:   "pod policy alone",
			labels: map[string]string{labelPodPolicy: "restricted"},
			want:   TemplateDefault,
		},
		{
			name:   "extended monitoring alone",
			labels: map[string]string{labelExtendedMonitoring: ""},
			want:   TemplateDefault,
		},
		{
			name:   "vulnerability scanning wins over the rest",
			labels: map[string]string{labelSecurityScanning: "", labelPodPolicy: "baseline"},
			want:   TemplateSecure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TemplateFor(namespace("foo", tt.labels, nil)))
		})
	}
}

func TestParametersFor(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		template    string
		want        map[string]any
	}{
		{
			name:     "simple without user metadata carries no parameters",
			template: TemplateSimple,
			want:     nil,
		},
		{
			name:        "simple mirrors user metadata only",
			labels:      map[string]string{"team": "blue", v1alpha3.ResourceLabelProject: "foo"},
			annotations: map[string]string{"owner": "alice"},
			template:    TemplateSimple,
			want: map[string]any{
				"namespace": map[string]any{
					"labels":      map[string]any{"team": "blue"},
					"annotations": map[string]any{"owner": "alice"},
				},
			},
		},
		{
			name:     "default keeps the namespace as permissive as it is today",
			labels:   map[string]string{labelExtendedMonitoring: ""},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfilePrivileged,
				"extendedMonitoringEnabled": true,
				"requiredRequests":          false,
			},
		},
		{
			name:     "default reads the profile back from the rendered label",
			labels:   map[string]string{labelPodPolicy: "restricted"},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileRestricted,
				"extendedMonitoringEnabled": false,
				"requiredRequests":          false,
			},
		},
		{
			name:     "secure adds the scanning switch",
			labels:   map[string]string{labelSecurityScanning: "", labelPodPolicy: "baseline"},
			template: TemplateSecure,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileBaseline,
				"extendedMonitoringEnabled": false,
				"securityScanningEnabled":   true,
				"requiredRequests":          false,
			},
		},
		{
			name:     "rendered labels are not mirrored as user metadata",
			labels:   map[string]string{labelPodPolicy: "baseline", labelExtendedMonitoring: ""},
			template: TemplateDefault,
			want: map[string]any{
				"networkPolicy":             networkPolicyNotRestricted,
				"podSecurityProfile":        podSecurityProfileBaseline,
				"extendedMonitoringEnabled": true,
				"requiredRequests":          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParametersFor(namespace("foo", tt.labels, tt.annotations), tt.template)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPodSecurityProfile(t *testing.T) {
	tests := []struct {
		name  string
		label string
		want  string
	}{
		{name: "missing label falls back to the permissive profile", label: "", want: podSecurityProfilePrivileged},
		{name: "unknown value falls back to the permissive profile", label: "whatever", want: podSecurityProfilePrivileged},
		{name: "baseline", label: "baseline", want: podSecurityProfileBaseline},
		{name: "restricted", label: "restricted", want: podSecurityProfileRestricted},
		{name: "privileged", label: "privileged", want: podSecurityProfilePrivileged},
		{name: "value casing is ignored", label: "ReStRiCtEd", want: podSecurityProfileRestricted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, podSecurityProfile(tt.label))
		})
	}
}

func TestFilterUserMeta(t *testing.T) {
	got := filterUserMeta(map[string]string{
		"team":                         "blue",
		"heritageSomething":            "keep",
		v1alpha3.ResourceLabelProject:  "foo",
		v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy,
		"kubernetes.io/metadata.name":  "foo",
		"meta.helm.sh/release-name":    "foo",
		labelPodPolicy:                 "baseline",
		labelExtendedMonitoring:        "",
		labelSecurityScanning:          "",
	})
	assert.Equal(t, map[string]string{"team": "blue", "heritageSomething": "keep"}, got)
}

func TestFilterUserMeta_NilWhenNothingLeft(t *testing.T) {
	assert.Nil(t, filterUserMeta(map[string]string{v1alpha3.ResourceLabelProject: "foo"}))
	assert.Nil(t, filterUserMeta(nil))
}

func TestNeedsTemplate(t *testing.T) {
	tests := []struct {
		name    string
		project *v1alpha3.Project
		want    bool
	}{
		{
			name:    "project left over from the namespace-managed model",
			project: project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, ""),
			want:    true,
		},
		{
			name:    "project created without a template",
			project: project("foo", nil, ""),
			want:    true,
		},
		{
			name:    "both at once",
			project: project("foo", map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}, ""),
			want:    true,
		},
		{
			name:    "already migrated",
			project: project("foo", nil, TemplateSimple),
			want:    false,
		},
		{
			name:    "virtual project is platform-owned",
			project: project("deckhouse", map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}, "virtual"),
			want:    false,
		},
		{
			name: "virtual project without a template is still skipped",
			project: project("default", map[string]string{
				v1alpha3.ProjectLabelVirtualProject: "true",
			}, ""),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needsTemplate(tt.project))
		})
	}
}
