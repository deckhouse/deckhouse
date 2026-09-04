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

package fencingstate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	jsonpatch "github.com/evanphx/json-patch/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/yaml"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

const crdPath = "../../../../../../crds/fencingfailednodestates.yaml"

const storedVersion = "v1alpha1"

var crdValidator = sync.OnceValues(func() (*validate.SchemaValidator, error) {
	raw, err := os.ReadFile(crdPath)
	if err != nil {
		return nil, err
	}

	asJSON, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, err
	}

	var crd struct {
		Spec struct {
			Versions []struct {
				Name   string `json:"name"`
				Schema struct {
					OpenAPIV3Schema json.RawMessage `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(asJSON, &crd); err != nil {
		return nil, err
	}

	for _, version := range crd.Spec.Versions {
		if version.Name != storedVersion {
			continue
		}

		var schema spec.Schema
		if err := json.Unmarshal(version.Schema.OpenAPIV3Schema, &schema); err != nil {
			return nil, err
		}

		return validate.NewSchemaValidator(&schema, nil, "", strfmt.Default), nil
	}

	return nil, fmt.Errorf("%s: no version %s", crdPath, storedVersion)
})

func apiServerVerdict(t *testing.T, base *v1alpha1.FencingFailedNodeState, patch []byte) []error {
	t.Helper()

	validator, err := crdValidator()
	if err != nil {
		t.Fatalf("load %s: %v", crdPath, err)
	}

	object := base.DeepCopy()
	object.APIVersion = v1alpha1.GroupVersion.String()
	object.Kind = "FencingFailedNodeState"

	original, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("encode the stored object: %v", err)
	}

	merged, err := jsonpatch.MergePatch(original, patch)
	if err != nil {
		t.Fatalf("apply the merge patch %s: %v", patch, err)
	}

	var document map[string]any
	if err := utiljson.Unmarshal(merged, &document); err != nil {
		t.Fatalf("decode the patched object: %v", err)
	}

	return validator.Validate(document).Errors
}

func crdBaseObject() *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testPeerName,
			Labels: map[string]string{domain.NodeGroupLabel: testNodeGroup},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: nodeAPIVersion,
				Kind:       nodeKind,
				Name:       testPeerName,
				UID:        types.UID(testPeerUID),
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  testNodeGroup,
			ProfileRef: v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
	}
}

func TestAPIServerAcceptsTheHeartbeatThisAdapterSends(t *testing.T) {
	var sent []byte

	states, _ := newStates(t, interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, c client.Client, name string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			data, err := patch.Data(obj)
			if err != nil {
				return err
			}

			sent = data

			return c.SubResource(name).Patch(ctx, obj, patch, opts...)
		},
	}, crdBaseObject())

	if err := states.Heartbeat(t.Context(), testPeerName, fallbackSection()); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	if len(sent) == 0 {
		t.Fatal("no patch reached the client")
	}

	if errs := apiServerVerdict(t, crdBaseObject(), sent); len(errs) > 0 {
		t.Errorf("the api server would reject the heartbeat %s: %v", sent, errs)
	}
}

func TestAPIServerHoldsTheHeartbeatToMicroseconds(t *testing.T) {
	section := func(at string) []byte {
		return []byte(`{"status":{"fallback":{"active":true,"apiReachable":true,"heartbeatInterval":"250ms","lastHeartbeatAt":"` + at + `"}}}`)
	}

	if errs := apiServerVerdict(t, crdBaseObject(), section("2026-06-02T15:00:02.250000Z")); len(errs) > 0 {
		t.Errorf("the api server rejected a microsecond timestamp: %v", errs)
	}

	for name, at := range map[string]string{
		"whole seconds": "2026-06-02T15:00:02Z",
		"milliseconds":  "2026-06-02T15:00:02.250Z",
		"nanoseconds":   "2026-06-02T15:00:02.250000000Z",
	} {
		t.Run(name, func(t *testing.T) {
			errs := apiServerVerdict(t, crdBaseObject(), section(at))
			if len(errs) == 0 {
				t.Fatalf("the api server accepted lastHeartbeatAt %q, want the pattern to refuse it", at)
			}

			if got := errs[0].Error(); !strings.Contains(got, "lastHeartbeatAt") {
				t.Errorf("rejected with %q, want the complaint to be about lastHeartbeatAt", got)
			}
		})
	}
}
