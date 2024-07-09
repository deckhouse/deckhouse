package manifests

import (
	"testing"

	pkg_utils "system-registry-manager/pkg/utils"
)

func TestChangePodAnnotations(t *testing.T) {
	tests := []struct {
		name           string
		manifest       []byte
		newAnnotations map[string]string
		expectedResult []byte
		expectedError  error
	}{
		{
			name: "Change annotations",
			manifest: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations:
    old_annotation: old_value
spec:
  containers:
  - name: nginx
    image: nginx:latest
`),
			newAnnotations: map[string]string{
				"new_annotation": "new_value",
			},
			expectedResult: []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  annotations:
    old_annotation: old_value
    new_annotation: new_value
spec:
  containers:
  - name: nginx
    image: nginx:latest
`),
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ChangePodAnnotations(tt.manifest, tt.newAnnotations)

			if (err != nil && tt.expectedError == nil) || (err == nil && tt.expectedError != nil) {
				t.Errorf("Error mismatch, expected: %v, got: %v", tt.expectedError, err)
			}

			if !pkg_utils.EqualYaml(result, tt.expectedResult) {
				t.Errorf("Result mismatch, expected: %s, got: %s", string(tt.expectedResult), string(result))
			}
		})
	}
}
