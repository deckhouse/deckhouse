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
	"math/rand"

	"d8.io/upmeter/pkg/check"
)

func Fake() check.Checker {
	return &fakeChecker{}
}

type fakeChecker struct {
	current check.Error
	hold    int
}

var errPool = []check.Error{nil, nil, nil, nil, check.ErrFail("fail"), check.ErrUnknown("unknown")}

func (c *fakeChecker) Check() check.Error {
	if c.hold > 0 {
		c.hold--
		return c.current
	}
	// pick random error for some time
	i := rand.Int() % len(errPool)
	c.current = errPool[i]
	c.hold = rand.Int() % 600
	return c.current
}
