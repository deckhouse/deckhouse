package checker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

func TestKubeControllerObjectLifecycle_Check(t *testing.T) {
	type fields struct {
		preflight doer

		parentGetter  doer
		parentCreator doer
		parentDeleter doer

		childPresenceGetter doer
		childAbsenceGetter  doer
		childDeleter        doer
	}
	tests := []struct {
		name   string
		fields fields
		want   check.Status

		parentDeletions int
		childDeletions  int
	}{
		{
			name: "Clean run without garbage",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Up,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Found parent garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  &successDoer{},
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Found child garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					nil, // present garbage
					nil, // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 0,
			childDeletions:  1,
		},
		{
			name: "Failed preflight results in Unknown ",
			fields: fields{
				preflight:     doerErr("no version"),
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 0,
			childDeletions:  0,
		},
		{
			name: "Arbitrary error while getting parent results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doerErr("getting parent"),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 0,
			childDeletions:  0,
		},
		{
			name: "Arbitrary error while getting child results in Unknown (parent cleaned)",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Arbitrary error while getting child garbage results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					fmt.Errorf("getting child"), // arbitrary error
					nil,                         // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 0,
			childDeletions:  0,
		},
		{
			name: "Arbitrary error while getting child presence results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Arbitrary error while getting child absence results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: doerErr("getting child absence"),
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Arbitrary parent creation error results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: doerErr("creating"),
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 0,
			childDeletions:  0,
		},
		{
			name: "Arbitrary parent deletion error results in Unknown",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(fmt.Errorf("creating")),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(nil),
			},
			want:            check.Unknown,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Arbitrary child deletion error has no effect in happy case flow (not called)",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: &successDoer{},
				childDeleter:       newSequenceDoer(fmt.Errorf("deleting child")),
			},
			want:            check.Up,
			parentDeletions: 1,
			childDeletions:  0,
		},
		{
			name: "Arbitrary child deletion error has no effect in child object cleanup (fail prioritized)",
			fields: fields{
				preflight:     &successDoer{},
				parentGetter:  doer404(),
				parentCreator: &successDoer{},
				parentDeleter: newSequenceDoer(nil),
				childPresenceGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
				),
				childAbsenceGetter: doer404(),
				childDeleter:       newSequenceDoer(fmt.Errorf("deleting child")),
			},
			want:            check.Down,
			parentDeletions: 1,
			childDeletions:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubeControllerObjectLifecycle{
				preflight:           tt.fields.preflight,
				parentGetter:        tt.fields.parentGetter,
				parentCreator:       tt.fields.parentCreator,
				parentDeleter:       tt.fields.parentDeleter,
				childPresenceGetter: tt.fields.childPresenceGetter,
				childAbsenceGetter:  tt.fields.childAbsenceGetter,
				childDeleter:        tt.fields.childDeleter,
			}

			err := c.Check()
			assertCheckStatus(t, tt.want, err)

			assert.Equal(t, tt.parentDeletions, tt.fields.parentDeleter.(*sequenceDoer).i,
				"Unexpected number of parent GC calls")

			assert.Equal(t, tt.childDeletions, tt.fields.childDeleter.(*sequenceDoer).i,
				"Unexpected number of child GC calls")
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
