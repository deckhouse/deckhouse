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

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSaverWithEmptyDestNotStartWatcher(t *testing.T) {
	tests := []struct {
		name         string
		destinations []SaverDestination
	}{
		{
			name:         "nil array",
			destinations: nil,
		},
		{
			name:         "empty array",
			destinations: []SaverDestination{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			saver := NewStateSaver(tc.destinations)
			require.NotNil(t, saver)
			require.Len(t, saver.saversDestinations, 0)

			err := saver.Start(newTestRunnerWithChanges())

			require.NoError(t, err)
			require.False(t, saver.IsStarted())
			require.Nil(t, saver.watcher)
		})
	}
}
