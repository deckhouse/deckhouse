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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"d8.io/upmeter/pkg/check"
)

// KubeControllerObjectLifecycle checks controller object lifecycle where a
// creation of parent object leads to the creation of a child one, and the same
// with deletion.
type KubeControllerObjectLifecycle struct {
	preflight Doer

	parentGetter  Doer
	parentCreator Doer
	parentDeleter Doer

	childGetter          Doer
	childDeleter         Doer
	childPollingInterval time.Duration
	childPollingTimeout  time.Duration
}

func (c *KubeControllerObjectLifecycle) Check() check.Error {
	ctx := context.TODO()
	if err := c.preflight.Do(ctx); err != nil {
		return check.ErrUnknown("preflight: %v", err)
	}

	if err := c.cleanGarbage(ctx, c.parentGetter, c.parentDeleter); err != nil {
		return check.ErrUnknown(err.Error())
	}
	if err := c.cleanGarbage(ctx, c.childGetter, c.childDeleter); err != nil {
		return check.ErrUnknown(err.Error())
	}

	// 1. create parent
	if createErr := c.parentCreator.Do(ctx); createErr != nil {
		return check.ErrUnknown("creating parent: %v", createErr)
	}

	// 2. expect child
	if getErr := c.childGetterUntilPresent().Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		_ = c.parentDeleter.Do(ctx) // Cleanup
		return check.ErrUnknown("getting child: %v", getErr)
	} else if apierrors.IsNotFound(getErr) {
		_ = c.parentDeleter.Do(ctx) // Cleanup
		return check.ErrFail("verification: child is not present after parent was created")
	}

	// 3. delete parent
	if delErr := c.parentDeleter.Do(ctx); delErr != nil {
		return check.ErrUnknown("deleting parent: %v", delErr)
	}

	// 4. expect no child
	if getErr := c.childGetterUntilAbsent().Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		return check.ErrUnknown("getting child: %v", getErr)
	} else if getErr == nil {
		_ = c.childDeleter.Do(ctx) // Cleanup
		return check.ErrFail("verification: child is present after parent was deleted")
	}

	return nil
}

func (c *KubeControllerObjectLifecycle) cleanGarbage(ctx context.Context, getter, deleter Doer) error {
	if getErr := getter.Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		return fmt.Errorf("getting garbage: %v", getErr)
	} else if getErr == nil {
		// Garbage found, clean and skip.
		if delErr := deleter.Do(ctx); delErr != nil {
			return fmt.Errorf("deleting garbage: %v", delErr)
		}
		return fmt.Errorf("cleaned garbage")
	}
	return nil
}

func (c *KubeControllerObjectLifecycle) childGetterUntilPresent() Doer {
	return &pollingDoer{
		doer:     c.childGetter,
		catch:    isNil,
		timeout:  c.childPollingTimeout,
		interval: c.childPollingInterval,
	}
}

func (c *KubeControllerObjectLifecycle) childGetterUntilAbsent() Doer {
	return &pollingDoer{
		doer:     c.childGetter,
		catch:    apierrors.IsNotFound,
		timeout:  c.childPollingTimeout,
		interval: c.childPollingInterval,
	}
}

func isNil(err error) bool { return err == nil }

type pollingDoer struct {
	doer     Doer
	catch    func(error) bool
	timeout  time.Duration
	interval time.Duration
}

func (p *pollingDoer) Do(ctx context.Context) (err error) {
	deadline := time.NewTimer(p.timeout)
	ticker := time.NewTicker(p.interval)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err = p.doer.Do(ctx)
			if p.catch(err) {
				return err
			}
		case <-deadline.C:
			return err
		}
	}
}
