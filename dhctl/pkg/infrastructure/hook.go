// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import "context"

type InfraActionHook interface {
	BeforeAction(context.Context, RunnerInterface) (runAfterAction bool, err error)
	IsReady() error
	AfterAction(context.Context, RunnerInterface) error
}

type DummyHook struct{}

func (c *DummyHook) BeforeAction(context.Context, RunnerInterface) (runPostAction bool, err error) {
	return false, nil
}

func (c *DummyHook) IsReady() error {
	return nil
}

func (c *DummyHook) AfterAction(context.Context, RunnerInterface) error {
	return nil
}
