package checker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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

func doerErr(msg string) *failDoer {
	return &failDoer{err: fmt.Errorf(msg)}
}

func doer404() *failDoer {
	err := apierrors.NewNotFound(schema.GroupResource{}, "")
	return &failDoer{err: err}
}

func TestKubeObjectBasicLifecycle_Check(t *testing.T) {
	type fields struct {
		preflight doer
		creator   doer
		getter    doer
		deleter   doer
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
			if tt.want == check.Up {
				assert.NoError(t, err, "Expected no err")
			} else {
				var got check.Status
				if err == nil {
					got = check.Up
				} else {
					got = err.Status()
				}
				assert.Equal(t, tt.want.String(), got.String())
			}
		})
	}
}
