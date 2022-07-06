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
	"testing"
)

type successfulPodNodeFetcher struct {
	node string
}

func (f *successfulPodNodeFetcher) Node(_ context.Context) (string, error) {
	return f.node, nil
}

type failingPodNodeFetcher struct {
	err error
}

func (f *failingPodNodeFetcher) Node(_ context.Context) (string, error) {
	return "", f.err
}

func TestPodPhaseChecker_Check(t *testing.T) {
	type fields struct {
		preflight    doer
		getter       doer
		creator      doer
		deleter      doer
		phaseFetcher podNodeFetcher
		node         string
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "a"},
				node:         "a",
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
				phaseFetcher: &failingPodNodeFetcher{err: fmt.Errorf("cannot fetch")},
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "y"}, // unexpected node
				node:         "a",
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
				phaseFetcher: &successfulPodNodeFetcher{node: "y"}, // unexpected node
				node:         "a",
			},
			want: check.Down,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &podPhaseChecker{
				preflight:   tt.fields.preflight,
				creator:     tt.fields.creator,
				getter:      tt.fields.getter,
				deleter:     tt.fields.deleter,
				nodeFetcher: tt.fields.phaseFetcher,
				node:        tt.fields.node,
			}

			err := c.Check()
			assertCheckStatus(t, tt.want, err)
		})
	}
}
