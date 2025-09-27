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

package server

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncWriter_Write(t *testing.T) {
	t.Parallel()

	var (
		buf bytes.Buffer
		wg  sync.WaitGroup
	)

	sw := &syncWriter{writer: &buf}
	numWriters := 100
	iterations := 100

	for i := range numWriters {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for j := range iterations {
				_, err := sw.Write([]byte(fmt.Sprintf("writer%d_iter%d\t", id, j)))
				require.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	lines := strings.Split(buf.String(), "\t")
	assert.Equal(t, numWriters*iterations+1, len(lines))
}

func TestSyncWriter_copyFrom(t *testing.T) {
	t.Parallel()

	var (
		buf     bytes.Buffer
		wg      sync.WaitGroup
		dataLen atomic.Int64
	)

	sw := &syncWriter{writer: &buf}
	numCopiers := 100

	for i := range numCopiers {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			data := fmt.Sprintf("data from copier %d", id)
			dataLen.Add(int64(len(data)))

			err := sw.copyFrom(strings.NewReader(data))
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, dataLen.Load(), int64(buf.Len()))
}

func TestSyncWriter_copyFrom_LargeData(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	sw := &syncWriter{writer: &buf}
	largeData := strings.Repeat("ABCDE", 1024*1024) // 5MB
	reader := strings.NewReader(largeData)

	done := make(chan error, 1)
	go func() {
		done <- sw.copyFrom(reader)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.Fail(t, "copyFrom timeout")
	}

	assert.Equal(t, largeData, buf.String())
}

func TestSyncWriters(t *testing.T) {
	var (
		stdoutBuf, stderrBuf bytes.Buffer
		wg                   sync.WaitGroup
	)

	writesNum := 100
	sw := &syncWriters{
		stdoutWriter: &syncWriter{writer: &stdoutBuf},
		stderrWriter: &syncWriter{writer: &stderrBuf},
	}

	wg.Add(2)

	go func() {
		defer wg.Done()
		for range writesNum {
			_, err := sw.stdoutWriter.Write([]byte("stdout"))
			require.NoError(t, err)
		}
	}()

	go func() {
		defer wg.Done()
		for range writesNum {
			_, err := sw.stderrWriter.Write([]byte("stderr"))
			require.NoError(t, err)
		}
	}()

	wg.Wait()

	assert.Equal(t, strings.Repeat("stdout", writesNum), stdoutBuf.String())
	assert.Equal(t, strings.Repeat("stderr", writesNum), stderrBuf.String())
}
