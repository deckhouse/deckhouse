/*
Copyright 2021 Flant JSC

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

package checker

import (
	"sigs.k8s.io/yaml"
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

func Test_certificateManifest(t *testing.T) {
	manifest := certificateManifest("xyz", "big-xyz", "somens")

	// agentID, name, namespace string
	expected := `
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    heritage: upmeter
    upmeter-agent: "xyz"
    upmeter-group: control-plane
    upmeter-probe: cert-manager
  name: "big-xyz"
  namespace: "somens"
spec:
  certificateOwnerRef: true
  dnsNames:
  - nothing-xyz.example.com
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
  secretName: "big-xyz"
  secretTemplate:
    labels:
      heritage: upmeter
      upmeter-agent: "xyz"
      upmeter-group: control-plane
      upmeter-probe: cert-manager
`
	assert.Equal(t, expected, manifest)

	var obj map[string]interface{}
	err := yaml.Unmarshal([]byte(manifest), obj)
	assert.NoError(t, err, "YAML is expected to be valid")
}

// dummyChecker is for mocking checkers
type dummyChecker struct{ err check.Error }

func (c *dummyChecker) Check() check.Error { return c.err }

func fallback(v, f check.Checker) check.Checker {
	if v != nil {
		return v
	}
	return f
}

var checkUp = &dummyChecker{nil}

func checkDown(msg string) check.Checker {
	return &dummyChecker{check.ErrFail(msg)}
}

func checkUnknown(msg string) check.Checker {
	return &dummyChecker{check.ErrUnknown(msg)}
}

func TestCertificateSecretLifecycleChecker2_Check(t *testing.T) {
	type fields struct {
		controlPlanePreflight   check.Checker
		garbagePreflight        check.Checker
		createCertOrUnknown     check.Checker
		getSecretPresenceOrFail check.Checker
		deleteCertFinalizer     check.Checker
		getSecretAbsenceOrFail  check.Checker
	}
	tests := []struct {
		name   string
		fields fields
		want   check.Error
	}{
		{
			name: "all success",
			fields: fields{
				controlPlanePreflight:   checkUp,
				garbagePreflight:        checkUp,
				createCertOrUnknown:     checkUp,
				getSecretPresenceOrFail: checkUp,
				deleteCertFinalizer:     checkUp,
				getSecretAbsenceOrFail:  checkUp,
			},
			want: nil,
		},
		{
			name: "control plane unavailable leads to unknown",
			fields: fields{
				controlPlanePreflight: checkUnknown("cp unavailable"),
			},
			want: check.ErrUnknown("cp unavailable"),
		},
		{
			name: "control plane unavailable",
			fields: fields{
				garbagePreflight: checkUnknown("garbage cleaned from previous run"),
			},
			want: check.ErrUnknown("garbage cleaned from previous run"),
		},
		{
			name: "control plane unavailable",
			fields: fields{
				garbagePreflight: checkUnknown("garbage cleaned from previous run"),
			},
			want: check.ErrUnknown("garbage cleaned from previous run"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CertificateSecretLifecycleChecker2{
				controlPlanePreflight:   fallback(tt.fields.controlPlanePreflight, checkUp),
				garbagePreflight:        fallback(tt.fields.garbagePreflight, checkUp),
				createCertOrUnknown:     fallback(tt.fields.createCertOrUnknown, checkUp),
				getSecretPresenceOrFail: fallback(tt.fields.getSecretPresenceOrFail, checkUp),
				deleteCertFinalizer:     fallback(tt.fields.deleteCertFinalizer, checkUp),
				getSecretAbsenceOrFail:  fallback(tt.fields.getSecretAbsenceOrFail, checkUp),
			}
			assert.Equalf(t, tt.want, c.Check(), "Check()")
		})
	}
}
