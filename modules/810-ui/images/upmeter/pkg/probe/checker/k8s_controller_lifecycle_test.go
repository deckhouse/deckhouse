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
	"time"

	"github.com/stretchr/testify/assert"

	"d8.io/upmeter/pkg/check"
)

func TestKubeControllerObjectLifecycle_Check(t *testing.T) {
	type fields struct {
		preflight Doer

		parentGetter  Doer
		parentCreator Doer
		parentDeleter Doer

		childGetter  Doer
		childDeleter Doer
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					nil,    // present garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
					err404,                      // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					fmt.Errorf("getting child"), // arbitrary error
					nil,                         // presence
					err404,                      // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404,                      // absence, no garbage
					fmt.Errorf("getting child"), // arbitrary error
					err404,                      // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404,                        // absence, no garbage
					nil,                           // presence
					fmt.Errorf("something wrong"), // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(nil),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					err404, // absence
				),
				childDeleter: newSequenceDoer(fmt.Errorf("deleting child")),
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
				childGetter: newSequenceDoer(
					err404, // absence, no garbage
					nil,    // presence
					nil,    // still presence, should lead to fail
				),
				childDeleter: newSequenceDoer(fmt.Errorf("deleting child")),
			},
			want:            check.Down,
			parentDeletions: 1,
			childDeletions:  1,
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
				// ensure ticker runs earlier than timeout
				childPollingInterval: time.Millisecond / 2,
				childPollingTimeout:  time.Millisecond,
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
	if d.i >= len(d.errors) {
		// stick to last error
		return d.errors[d.i-1]
	}

	err := d.errors[d.i]
	d.i++
	return err
}
