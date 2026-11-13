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

package systemnetwork

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type readerFunc func(ctx context.Context, key client.ObjectKey, obj client.Object) error

func (f readerFunc) Get(ctx context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	return f(ctx, key, obj)
}

func (readerFunc) List(context.Context, client.ObjectList, ...client.ListOption) error {
	panic("the webhook does not list")
}

var systemNetworkResource = schema.GroupResource{Group: "network.deckhouse.io", Resource: "systemnetworks"}

func moduleConfig(settings string) []byte {
	return []byte(`{"apiVersion":"deckhouse.io/v1alpha1","kind":"ModuleConfig","metadata":{"name":"node-manager"},"spec":{"settings":` + settings + `}}`)
}

func networkSettings(value string) []byte {
	return moduleConfig(`{"fencing":{"systemNetworkFallback":{"systemNetworkName":` + value + `}}}`)
}

func review(op admissionv1.Operation, object, oldObject []byte) admission.Request {
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: op}}
	req.Object.Raw = object
	req.OldObject.Raw = oldObject
	return req
}

func TestHandle(t *testing.T) {
	found := func(context.Context, client.ObjectKey, client.Object) error { return nil }
	missing := func(_ context.Context, key client.ObjectKey, _ client.Object) error {
		return apierrors.NewNotFound(systemNetworkResource, key.Name)
	}
	// The API answers a missing resource with a status whose details name the object of the request.
	missingResource := func(_ context.Context, key client.ObjectKey, _ client.Object) error {
		err := apierrors.NewNotFound(systemNetworkResource, key.Name)
		err.ErrStatus.Details.Causes = []metav1.StatusCause{{Type: metav1.CauseTypeUnexpectedServerResponse}}
		return err
	}

	const (
		notFound     = "SystemNetwork `sysnet-a` not found"
		notInstalled = "SystemNetwork is not installed (is the sdn module disabled?)"
	)

	tests := []struct {
		name     string
		get      readerFunc
		op       admissionv1.Operation
		object   []byte
		old      []byte
		wantDeny string // a part of the message; empty: the request is allowed
		wantGet  bool
	}{
		{name: "no name", get: found, op: admissionv1.Create, object: moduleConfig(`{}`)},
		{name: "empty name", get: found, op: admissionv1.Update, object: networkSettings(`""`), old: networkSettings(`"sysnet-a"`)},
		{name: "null name", get: found, op: admissionv1.Update, object: networkSettings(`null`), old: networkSettings(`"sysnet-a"`)},
		{name: "same name as before", get: missing, op: admissionv1.Update, object: networkSettings(`"sysnet-a"`), old: networkSettings(`"sysnet-a"`)},
		{name: "network exists", get: found, op: admissionv1.Update, object: networkSettings(`"sysnet-a"`), old: networkSettings(`"sysnet-b"`), wantGet: true},
		{name: "network exists on create", get: found, op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantGet: true},
		{name: "network does not exist", get: missing, op: admissionv1.Update, object: networkSettings(`"sysnet-a"`), old: networkSettings(`""`), wantDeny: notFound, wantGet: true},
		{name: "network does not exist on create", get: missing, op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: notFound, wantGet: true},
		{
			name: "group and version are not served",
			get: func(context.Context, client.ObjectKey, client.Object) error {
				return &apiutil.ErrResourceDiscoveryFailed{{Group: "network.deckhouse.io", Version: "v1alpha1"}: apierrors.NewNotFound(schema.GroupResource{}, "")}
			},
			op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: notInstalled, wantGet: true,
		},
		{
			name: "group is served, kind is not",
			get: func(context.Context, client.ObjectKey, client.Object) error {
				return &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "network.deckhouse.io", Kind: "SystemNetwork"}, SearchedVersions: []string{"v1alpha1"}}
			},
			op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: notInstalled, wantGet: true,
		},
		{
			name: "scheme does not know the kind",
			get: func(context.Context, client.ObjectKey, client.Object) error {
				return runtime.NewNotRegisteredErrForKind("scheme", schema.GroupVersionKind{Kind: "SystemNetwork"})
			},
			op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: notInstalled, wantGet: true,
		},
		{name: "CRD deleted after the first read", get: missingResource, op: admissionv1.Update, object: networkSettings(`"sysnet-a"`), old: networkSettings(`""`), wantDeny: notInstalled, wantGet: true},
		{
			name: "no permission to read",
			get: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
				return apierrors.NewForbidden(systemNetworkResource, key.Name, errors.New("RBAC: access denied"))
			},
			op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: "RBAC: access denied", wantGet: true,
		},
		{
			name: "API does not answer",
			get: func(context.Context, client.ObjectKey, client.Object) error {
				return apierrors.NewServiceUnavailable("etcd is not ready")
			},
			op: admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: "etcd is not ready", wantGet: true,
		},
		{name: "number", get: found, op: admissionv1.Create, object: networkSettings(`5`), wantDeny: "expected a string"},
		{name: "object instead of string", get: found, op: admissionv1.Update, object: networkSettings(`{"a":"b"}`), old: networkSettings(`"sysnet-a"`), wantDeny: "expected a string"},
		{name: "fencing is not an object", get: found, op: admissionv1.Create, object: moduleConfig(`{"fencing":"text"}`), wantDeny: "fencing"},
		{name: "not JSON", get: found, op: admissionv1.Create, object: []byte(`{`), wantDeny: "ModuleConfig"},
		{
			name: "handler panics",
			get:  func(context.Context, client.ObjectKey, client.Object) error { panic("boom") },
			op:   admissionv1.Create, object: networkSettings(`"sysnet-a"`), wantDeny: "boom", wantGet: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gets := 0
			v := &validator{reader: readerFunc(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				gets++
				if key.Name != "sysnet-a" {
					return fmt.Errorf("asked for %q", key.Name)
				}
				return tc.get(ctx, key, obj)
			})}

			resp := v.Handle(t.Context(), review(tc.op, tc.object, tc.old))

			if tc.wantDeny == "" {
				if !resp.Allowed {
					t.Fatalf("denied: %v", resp.Result)
				}
			} else {
				if resp.Allowed {
					t.Fatal("allowed")
				}
				if !strings.Contains(resp.Result.Message, tc.wantDeny) {
					t.Fatalf("message %q does not contain %q", resp.Result.Message, tc.wantDeny)
				}
			}
			if (gets > 0) != tc.wantGet {
				t.Fatalf("reads of the network: %d, want read: %v", gets, tc.wantGet)
			}
		})
	}
}
