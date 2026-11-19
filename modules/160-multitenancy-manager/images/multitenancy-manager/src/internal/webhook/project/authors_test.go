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

package project

import (
	"context"
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

const (
	createdBy      = v1alpha3.ProjectAnnotationCreatedBy
	lastModifiedBy = v1alpha3.ProjectAnnotationLastModifiedBy
)

// rawProject is a project as the API server sends it, with a field the Project type does not know, so
// a test can tell that the mutating webhook leaves such a field alone.
func rawProject(annotations, labels map[string]any, parameters map[string]any, finalizers ...any) map[string]any {
	metadata := map[string]any{"name": "foo"}
	if annotations != nil {
		metadata["annotations"] = annotations
	}
	if labels != nil {
		metadata["labels"] = labels
	}
	if len(finalizers) > 0 {
		metadata["finalizers"] = finalizers
	}
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha3",
		"kind":       "Project",
		"metadata":   metadata,
		"spec": map[string]any{
			"projectTemplateName":  "default",
			"parameters":           parameters,
			"fieldOfALaterVersion": map[string]any{"kept": true},
		},
	}
}

func authorsRequest(t *testing.T, operation admissionv1.Operation, user string, old, updated map[string]any) admission.Request {
	t.Helper()
	raw, err := json.Marshal(updated)
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: operation,
		UserInfo:  authnv1.UserInfo{Username: user},
		Object:    runtime.RawExtension{Raw: raw},
	}}
	if old != nil {
		oldRaw, err := json.Marshal(old)
		require.NoError(t, err)
		req.OldObject = runtime.RawExtension{Raw: oldRaw}
	}
	return req
}

// mutate runs the mutating webhook and returns the project it admits.
func mutate(t *testing.T, req admission.Request) map[string]any {
	t.Helper()
	resp := authorsMutator{}.Handle(context.Background(), req)
	require.True(t, resp.Allowed, "the mutating webhook never refuses: %v", resp.Result)
	raw := req.Object.Raw
	if len(resp.Patches) > 0 {
		patchRaw, err := json.Marshal(resp.Patches)
		require.NoError(t, err)
		patch, err := jsonpatch.DecodePatch(patchRaw)
		require.NoError(t, err)
		raw, err = patch.Apply(raw)
		require.NoError(t, err)
	}
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestAuthorsMutator(t *testing.T) {
	t.Parallel()
	params := map[string]any{"a": "1"}
	both := map[string]any{createdBy: "alice", lastModifiedBy: "alice"}

	cases := []struct {
		name      string
		operation admissionv1.Operation
		user      string
		old       map[string]any
		updated   map[string]any
		want      map[string]any // the annotations of the admitted project; nil for none
	}{
		{
			name: "a create records the creator as both", operation: admissionv1.Create, user: "alice",
			updated: rawProject(nil, nil, params),
			want:    both,
		},
		{
			name: "a create overwrites forged values", operation: admissionv1.Create, user: "alice",
			updated: rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob", "team": "x"}, nil, params),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "alice", "team": "x"},
		},
		{
			name: "a change of the spec records the last to modify it", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(both, nil, map[string]any{"a": "2"}),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "bob"},
		},
		{
			name: "a change of a label is a change", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, map[string]any{"env": "dev"}, params),
			updated: rawProject(both, map[string]any{"env": "prod"}, params),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "bob"},
		},
		{
			name: "a change of another annotation is a change", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(map[string]any{createdBy: "alice", lastModifiedBy: "alice", "team": "x"}, nil, params),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "bob", "team": "x"},
		},
		{
			name: "a write of the finalizers alone is not a change", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(both, nil, params, "example.com/hold"),
			want:    both,
		},
		{
			name: "a new last applied configuration alone is not a change", operation: admissionv1.Update, user: "bob",
			old:     rawProject(map[string]any{createdBy: "alice", lastModifiedBy: "alice", corev1.LastAppliedConfigAnnotation: "{}"}, nil, params),
			updated: rawProject(map[string]any{createdBy: "alice", lastModifiedBy: "alice", corev1.LastAppliedConfigAnnotation: `{"x":1}`}, nil, params),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "alice", corev1.LastAppliedConfigAnnotation: `{"x":1}`},
		},
		{
			name: "removed values are put back", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(nil, nil, params),
			want:    both,
		},
		{
			name: "a content change restores the creator and records the modifier", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(nil, nil, map[string]any{"a": "2"}),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "bob"},
		},
		{
			name: "forged values are put back", operation: admissionv1.Update, user: "bob",
			old:     rawProject(both, nil, params),
			updated: rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob"}, nil, params),
			want:    both,
		},
		{
			name: "a project from before the webhook gets no creator", operation: admissionv1.Update, user: "bob",
			old:     rawProject(nil, nil, params),
			updated: rawProject(map[string]any{createdBy: "bob"}, nil, map[string]any{"a": "2"}),
			want:    map[string]any{lastModifiedBy: "bob"},
		},
		{
			name: "a project from before the webhook gets no author on a write that changes nothing", operation: admissionv1.Update, user: "bob",
			old:     rawProject(nil, nil, params),
			updated: rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob"}, nil, params),
			want:    nil,
		},
		{
			name: "a change made as system:sudouser is recorded", operation: admissionv1.Update, user: "system:sudouser",
			old:     rawProject(both, nil, params),
			updated: rawProject(both, nil, map[string]any{"a": "2"}),
			want:    map[string]any{createdBy: "alice", lastModifiedBy: "system:sudouser"},
		},
		{
			name: "the controller is not recorded", operation: admissionv1.Update, user: rolebindingwebhook.ControllerServiceAccount,
			old:     rawProject(both, nil, params),
			updated: rawProject(both, map[string]any{"projects.deckhouse.io/project": "foo"}, params),
			want:    both,
		},
		{
			name: "Deckhouse creating a project is not recorded", operation: admissionv1.Create, user: rolebindingwebhook.DeckhouseServiceAccount,
			updated: rawProject(nil, nil, params),
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			admitted := mutate(t, authorsRequest(t, tc.operation, tc.user, tc.old, tc.updated))

			// What the mutating webhook admits from a user must pass the check of the project webhook,
			// which refuses the write otherwise. The controller and Deckhouse skip that check.
			if !isPrivileged(tc.user) {
				reason, err := authorsRefusal(authorsRequest(t, tc.operation, tc.user, tc.old, admitted))
				require.NoError(t, err)
				assert.Empty(t, reason, "the project webhook accepts what the mutating webhook admits")
			}

			metadata, ok := admitted["metadata"].(map[string]any)
			require.True(t, ok)
			if tc.want == nil {
				assert.NotContains(t, metadata, "annotations")
			} else {
				assert.Equal(t, tc.want, metadata["annotations"])
			}
			assert.Equal(t, tc.updated["spec"], admitted["spec"], "the webhook changes the annotations only")
			assert.Equal(t, tc.updated["metadata"].(map[string]any)["labels"], metadata["labels"])
		})
	}
}

func TestAuthorsMutatorReinvocation(t *testing.T) {
	t.Parallel()

	params := map[string]any{"a": "1"}
	stored := map[string]any{createdBy: "alice", lastModifiedBy: "alice"}
	labels := map[string]any{"env": "dev"}
	old := rawProject(stored, labels, params)
	updated := rawProject(stored, nil, params)

	first := mutate(t, authorsRequest(t, admissionv1.Update, "bob", old, updated))
	metadata := first["metadata"].(map[string]any)
	require.Equal(t, "bob", metadata["annotations"].(map[string]any)[lastModifiedBy])

	// Another mutating webhook restores the original label.
	metadata["labels"] = labels
	reason, err := authorsRefusal(authorsRequest(t, admissionv1.Update, "bob", old, first))
	require.NoError(t, err)
	assert.Contains(t, reason, lastModifiedBy)

	second := mutate(t, authorsRequest(t, admissionv1.Update, "bob", old, first))
	assert.Equal(t, old, second)
	req := authorsRequest(t, admissionv1.Update, "bob", old, second)
	reason, err = authorsRefusal(req)
	require.NoError(t, err)
	assert.Empty(t, reason)
	assert.Equal(t, second, mutate(t, req))
}

func TestAuthorsRefusal(t *testing.T) {
	t.Parallel()
	params := map[string]any{"a": "1"}
	both := map[string]any{createdBy: "alice", lastModifiedBy: "alice"}

	cases := []struct {
		name      string
		operation admissionv1.Operation
		user      string
		old       map[string]any
		updated   map[string]any
		refused   string // the annotation the refusal names; "" when allowed
	}{
		{name: "a create with the values the webhook sets", operation: admissionv1.Create, user: "alice",
			updated: rawProject(both, nil, params)},
		{name: "a create the webhook did not reach", operation: admissionv1.Create, user: "alice",
			updated: rawProject(nil, nil, params)},
		{name: "a create naming another creator", operation: admissionv1.Create, user: "alice",
			updated: rawProject(map[string]any{createdBy: "bob"}, nil, params), refused: createdBy},
		{name: "a create naming another modifier", operation: admissionv1.Create, user: "alice",
			updated: rawProject(map[string]any{lastModifiedBy: "bob"}, nil, params), refused: lastModifiedBy},
		{name: "a change with the values the webhook sets", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "alice", lastModifiedBy: "bob"}, nil, map[string]any{"a": "2"})},
		{name: "a change the webhook did not reach", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "alice"}, nil, map[string]any{"a": "2"})},
		{name: "a change that keeps the previous modifier", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(both, nil, map[string]any{"a": "2"})},
		{name: "a change naming an unrelated modifier", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "alice", lastModifiedBy: "charlie"}, nil, map[string]any{"a": "2"}), refused: lastModifiedBy},
		{name: "a content change that removes the creator and keeps the previous modifier", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{lastModifiedBy: "alice"}, nil, map[string]any{"a": "2"})},
		{name: "a content change that removes the creator and records the current modifier", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{lastModifiedBy: "bob"}, nil, map[string]any{"a": "2"})},
		{name: "a content change that removes both authors", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(nil, nil, map[string]any{"a": "2"})},
		{name: "a write that only removes the creator", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{lastModifiedBy: "alice"}, nil, params)},
		{name: "an empty creator value is not an absent annotation", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "", lastModifiedBy: "alice"}, nil, params), refused: createdBy},
		{name: "a change naming the requester as a different creator", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob"}, nil, map[string]any{"a": "2"}), refused: createdBy},
		{name: "a change naming an unrelated creator", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "charlie", lastModifiedBy: "bob"}, nil, map[string]any{"a": "2"}), refused: createdBy},
		{name: "a missing creator does not allow an unrelated modifier", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{lastModifiedBy: "charlie"}, nil, map[string]any{"a": "2"}), refused: lastModifiedBy},
		{name: "the removal of both authors on a write that changes no content", operation: admissionv1.Update, user: "bob",
			old: rawProject(both, nil, params), updated: rawProject(nil, nil, params), refused: lastModifiedBy},
		{name: "the removal of the modifier on a write that changes nothing", operation: admissionv1.Update, user: "alice",
			old: rawProject(both, nil, params), updated: rawProject(map[string]any{createdBy: "alice"}, nil, params), refused: lastModifiedBy},
		{name: "a creator added to a project from before the webhook", operation: admissionv1.Update, user: "bob",
			old: rawProject(nil, nil, params), updated: rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob"}, nil, map[string]any{"a": "2"}), refused: createdBy},
		{name: "a project from before the webhook left as it is", operation: admissionv1.Update, user: "bob",
			old: rawProject(nil, nil, params), updated: rawProject(nil, nil, params)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reason, err := authorsRefusal(authorsRequest(t, tc.operation, tc.user, tc.old, tc.updated))
			require.NoError(t, err)
			if tc.refused == "" {
				assert.Empty(t, reason)
				return
			}
			assert.Contains(t, reason, tc.refused)
		})
	}
}

// The project webhook refuses a create that names another author.
func TestHandle_AuthorAnnotations(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()
	params := map[string]any{"a": "1"}
	forged := rawProject(map[string]any{createdBy: "bob", lastModifiedBy: "bob"}, nil, params)

	resp := v.Handle(ctx, authorsRequest(t, admissionv1.Create, "alice", nil, forged))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, createdBy)

}
