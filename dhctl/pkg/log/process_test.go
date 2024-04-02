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

package log

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/werf/logboek"
)

func TestProcessStack(t *testing.T) {
	s := &processStack{}

	s.push(&logProcessDescriptor{
		StartedAt: time.Now(),
		Msg:       "process1",
	})

	require.Len(t, s.activeProcesses, 1, "process1 is not added to stack")

	s.push(&logProcessDescriptor{
		StartedAt: time.Now(),
		Msg:       "process2",
	})

	require.Len(t, s.activeProcesses, 2, "process2 is not added to stack")

	assertPop := func(t *testing.T, len int, process string) {
		p := s.pop()

		require.NotNilf(t, p, "%s does not pop", process)
		require.Lenf(t, s.activeProcesses, len, "process1 does not remove from stack")
		require.Equalf(t, p.Msg, process, "incorrect process %s; should be %s", p.Msg, process)
	}

	assertPop(t, 1, "process2")
	assertPop(t, 0, "process1")

	p := s.pop()

	require.Nil(t, p, "returns none nil process from empty stack")
	require.Len(t, s.activeProcesses, 0, "pop from empty stack affect stack size")
}

func TestProcessLoggers(t *testing.T) {
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()

	loggers := []struct {
		logger ProcessLogger
		name   string
	}{
		{
			logger: newWrappedProcessLogger(&SilentLogger{}),
			name:   "wrapped logger",
		},

		{
			logger: newPrettyProcessLogger(logboek.DefaultLogger()),
			name:   "pretty logger",
		},
	}

	for _, l := range loggers {
		t.Run(l.name, func(t *testing.T) {
			t.Run("Do not panic done process without start", func(t *testing.T) {
				l.logger.LogProcessEnd()

				l.logger.LogProcessStart("process done")
				l.logger.LogProcessEnd()

				l.logger.LogProcessEnd()
			})

			t.Run("Do not panic failed process without start", func(t *testing.T) {
				l.logger.LogProcessFail()

				l.logger.LogProcessStart("process fail")
				l.logger.LogProcessFail()

				l.logger.LogProcessFail()
			})
		})
	}
}
