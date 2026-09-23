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
	// AfterAction runs whether or not the infrastructure action succeeded, and actionErr says
	// which: nil when it applied cleanly, otherwise the error it failed with.
	//
	// It is a parameter rather than something to look up because the distinction is easy to
	// forget and expensive to get wrong. A hook still has bookkeeping to do after a failed
	// action - a VM may have been recreated before the failure, and the session pinned to the
	// old address has to follow it - but anything that waits for the action's result must not
	// run: the wait cannot be satisfied by an action that did not happen, so it burns its whole
	// budget and then reports a second failure that only restates the first.
	AfterAction(ctx context.Context, runner RunnerInterface, actionErr error) error
}

type DummyHook struct{}

func (c *DummyHook) BeforeAction(context.Context, RunnerInterface) (bool, error) {
	return false, nil
}

func (c *DummyHook) IsReady() error {
	return nil
}

func (c *DummyHook) AfterAction(context.Context, RunnerInterface, error) error {
	return nil
}
