/*
Copyright 2021 Flant JSC

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
	"d8.io/upmeter/pkg/check"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"testing"
)

type successfulPodPhaseFetcher struct {
	phase v1.PodPhase
}

func (f *successfulPodPhaseFetcher) Fetch(_ context.Context) (v1.PodPhase, error) {
	return f.phase, nil
}

type failingPodPhaseFetcher struct {
	err error
}

func (f *failingPodPhaseFetcher) Fetch(_ context.Context) (v1.PodPhase, error) {
	return "", f.err
}

func TestPodPhaseChecker_Check(t *testing.T) {
	type fields struct {
		preflight    doer
		getter       doer
		creator      doer
		deleter      doer
		phaseFetcher podPhaseFetcher
		phase        v1.PodPhase
	}
	tests := []struct {
		name   string
		fields fields
		want   check.Status
	}{
		{
			name: "Clean run without garbage",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Up,
		},
		{
			name: "Found garbage results in Unknown",
			fields: fields{
				preflight:    &successDoer{},
				getter:       &successDoer{}, // no error means the object is found
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Failing preflight results in Unknown",
			fields: fields{
				preflight:    doerErr("no version"),
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary getting error results in Unknown",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doerErr("nope"),
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary creation error results in Unknown",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      doerErr("nope"),
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary deletion error results in Unknown",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      doerErr("nope"),
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodPending},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Arbitrary fetcher error results in Unknown",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &failingPodPhaseFetcher{err: fmt.Errorf("cannot fetch")},
				phase:        v1.PodPending,
			},
			want: check.Unknown,
		},
		{
			name: "Verification error results in fail",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      &successDoer{},
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodUnknown}, // enexpected phase
				phase:        v1.PodPending,
			},
			want: check.Down,
		},
		{
			name: "Arbitrary verification and deletion errors results in fail (verifier prioritized)",
			fields: fields{
				preflight:    &successDoer{},
				getter:       doer404(),
				creator:      &successDoer{},
				deleter:      doerErr("nope"),
				phaseFetcher: &successfulPodPhaseFetcher{phase: v1.PodUnknown}, // enexpected phase
				phase:        v1.PodPending,
			},
			want: check.Down,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &podPhaseChecker{
				preflight:    tt.fields.preflight,
				creator:      tt.fields.creator,
				getter:       tt.fields.getter,
				deleter:      tt.fields.deleter,
				phaseFetcher: tt.fields.phaseFetcher,
				phase:        tt.fields.phase,
			}

			err := c.Check()
			assertCheckStatus(t, tt.want, err)
		})
	}
}
