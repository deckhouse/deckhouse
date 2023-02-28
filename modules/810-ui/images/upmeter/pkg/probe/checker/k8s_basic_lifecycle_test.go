/*
Copyright 2023 Flant JSC

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
	"context"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"d8.io/upmeter/pkg/check"
)

// successDoer mocks a doer with that returns nil error
type successDoer struct{}

func (d *successDoer) Do(_ context.Context) error { return nil }

// failDoer mocks a doer with that returns specifier error
type failDoer struct{ err error }

func (d *failDoer) Do(_ context.Context) error { return d.err }

// doerErr creates doer that results in arbitrary error
func doerErr(msg string) *failDoer {
	return &failDoer{err: fmt.Errorf(msg)}
}

var err404 = apierrors.NewNotFound(schema.GroupResource{}, "")

// doer404 creates doer that results in kubernetes NotFound error
func doer404() *failDoer {
	return &failDoer{err: err404}
}

func TestKubeObjectBasicLifecycle_Check(t *testing.T) {
	type fields struct {
		preflight Doer
		getter    Doer
		creator   Doer
		deleter   Doer
	}
	tests := []struct {
		name   string
		fields fields
		want   check.Status
	}{
		{
			name: "Clean run without garbage",
			fields: fields{
				preflight: &successDoer{},
				getter:    doer404(),
				creator:   &successDoer{},
				deleter:   &successDoer{},
			},
			want: check.Up,
		},
		{
			name: "Found garbage results in Unknown",
			fields: fields{
				preflight: &successDoer{},
				getter:    &successDoer{}, // no error means the object is found
				creator:   &successDoer{},
				deleter:   &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Failing preflight results in Unknown",
			fields: fields{
				preflight: doerErr("no version"),
				getter:    doer404(),
				creator:   &successDoer{},
				deleter:   &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary getting error results in fail",
			fields: fields{
				preflight: &successDoer{},
				getter:    doerErr("nope"),
				creator:   &successDoer{},
				deleter:   &successDoer{},
			},
			want: check.Down,
		},
		{
			name: "Arbitrary creation error results in fail",
			fields: fields{
				preflight: &successDoer{},
				getter:    doer404(),
				creator:   doerErr("nope"),
				deleter:   &successDoer{},
			},
			want: check.Down,
		},
		{
			name: "Arbitrary deletion error results in fail",
			fields: fields{
				preflight: &successDoer{},
				getter:    doer404(),
				creator:   &successDoer{},
				deleter:   doerErr("nope"),
			},
			want: check.Down,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubeObjectBasicLifecycle{
				preflight: tt.fields.preflight,
				creator:   tt.fields.creator,
				getter:    tt.fields.getter,
				deleter:   tt.fields.deleter,
			}

			err := c.Check()
			assertCheckStatus(t, tt.want, err)
		})
	}
}
