package checker

import (
	"context"
	"fmt"
	"testing"

	"d8.io/upmeter/pkg/check"
)

func TestKubeControllerObjectLifecycle_Check(t *testing.T) {
	type fields struct {
		preflight     doer
		parentGetter  doer
		parentCreator doer
		parentDeleter doer
		childGetter   doer
		childDeleter  doer
	}
	tests := []struct {
		name   string
		fields fields
		want   check.Status
	}{
		{
			name: "Clean run without garbage",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Up,
		},
		{
			name: "Found parent garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  &successDoer{},
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Found child garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					nil,    // present garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Failed preflight results in Unknown ",
			fields: fields{
				preflight:     doerErr("no version"),
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary error while getting parent results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doerErr("getting parent"),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary error while getting child results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
					err404,                      // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary error while getting child garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					fmt.Errorf("getting child"), // arbitrary error
					nil,                         // presence
					err404,                      // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary error while getting child presence results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
					err404,                      // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary error while getting child absence results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					nil,                         // presence
					fmt.Errorf("getting child"), // arbitrary error
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary parent creation error results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: doerErr("creating"),
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary parent deletion error results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: doerErr("creating"),
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: &successDoer{},
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary child deletion error has no effect in happy case flow",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: doerErr("deleting child"),
			},
			want: check.Up,
		},
		{
			name: "Arbitrary child deletion error has no effect in child object cleanup (fail prioritized)",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: &successDoer{},
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					nil,    // unexpected presence
				),
				childDeleter: doerErr("deleting child"),
			},
			want: check.Down,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubeControllerObjectLifecycle{
				preflight:     tt.fields.preflight,
				parentGetter:  tt.fields.parentGetter,
				parentCreator: tt.fields.parentCreator,
				parentDeleter: tt.fields.parentDeleter,
				childGetter:   tt.fields.childGetter,
				childDeleter:  tt.fields.childDeleter,
			}

			err := c.Check()
			assertCheckStatus(t, tt.want, err)
		})
	}
}

type sequenceDoer struct {
	errors []error
	i      int
}

func newSequenceDoer(errors ...error) *sequenceDoer {
	return &sequenceDoer{errors: errors}
}

func (d *sequenceDoer) Do(_ context.Context) error {
	err := d.errors[d.i]
	d.i++
	return err
}
