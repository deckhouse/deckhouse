package config

import (
	"testing"

	"github.com/go-openapi/spec"
)

//nolint:funlen
func TestSchemaStore(t *testing.T) {
	newStore := SchemaStore{make(map[SchemaIndex]*spec.Schema)}

	err := newStore.upload([]byte(ClusterConfigSchema))
	if err != nil {
		t.Errorf("uploading error : %v", err)
	}

	err = newStore.upload([]byte(`
kind: TestKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    additionalProperties: false
    required: [kind, apiVersion, one, two]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      one:
        type: string
      two:
        type: string
`))
	if err != nil {
		t.Errorf("uploading error : %v", err)
	}

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			"Valid config",
			`
apiVersion: test
kind: TestKind
one: test
two: test
`,
			false,
		},
		{
			"Without version",
			`
kind: TestKind
one: "1"
two: "2"
`,
			true,
		},
		{
			"Without kind",
			`
apiVersion: test
one: "1"
two: "2"
`,
			true,
		},
		{
			"Wrong spec",
			`
apiVersion: test
kind: TestKind
one: "1"
`,
			true,
		},
	}

	for _, tc := range tests {
		content := []byte(tc.content)

		_, err := newStore.Validate(&content)
		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}
