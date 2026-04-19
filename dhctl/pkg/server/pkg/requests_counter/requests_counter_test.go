// Copyright 2024 Flant JSC
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

package requests_counter_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	rt "github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/requests_counter"
)

func TestRequestsCounter(t *testing.T) {
	numCurrentTasks := 2

	ctx, cancel := context.WithCancel(context.Background())

	// cancel context so that counter clean up runs only one time
	cancel()

	taskChan := make(chan struct{}, numCurrentTasks)

	counter := rt.New(time.Microsecond, taskChan)

	counter.Add("/dhctl.DHCTL/Check")
	counter.Add("/dhctl.DHCTL/Check")
	counter.Add("/dhctl.DHCTL/Converge")
	assert.Equal(t,
		map[string]int64{"/dhctl.DHCTL/Check": 2, "/dhctl.DHCTL/Converge": 1},
		counter.CountRecentRequests(),
	)

	for range numCurrentTasks {
		taskChan <- struct{}{}
	}

	counter.Run(ctx)

	<-time.After(time.Millisecond)

	assert.Equal(t,
		map[string]int64{"/dhctl.DHCTL/Check": 0, "/dhctl.DHCTL/Converge": 0},
		counter.CountRecentRequests(),
	)

	counter.Add("/dhctl.DHCTL/Check")
	counter.Add("/dhctl.DHCTL/Converge")
	assert.Equal(t,
		map[string]int64{"/dhctl.DHCTL/Check": 1, "/dhctl.DHCTL/Converge": 1},
		counter.CountRecentRequests(),
	)

	assert.Equal(t, numCurrentTasks, len(taskChan))
	assert.Equal(t, int64(numCurrentTasks), counter.CountCurrentRequests())
}
