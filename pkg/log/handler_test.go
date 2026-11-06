// Copyright 2026 Flant JSC
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

package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

// v0.2.1 lost this record: slog.JSONHandler dropped the empty group without its separator,
// the line did not decode and Handle returned the error.
func TestRecordWithDroppedGroupIsWritten(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBuffer(nil)
	l := log.NewLogger(log.WithOutput(buf), log.WithTimeFunc(func(time.Time) time.Time { return compatTime }))

	l.Info("m", "a", 1, slog.Group("g", "msg", "dropped"), "b", 2)
	l.Info("m", "a", 1, slog.Group("g", "source", "dropped below debug"), "b", 2)

	want := `{"level":"info","msg":"m","a":1,"b":2,"time":"2006-01-02T15:04:05+03:00"}` + "\n"
	assert.Equal(t, want+want, buf.String())
}

// logViaWrapper logs like addon-operator's adapters do: the custom key context
// makes the caller of the wrapper the source.
func logViaWrapper(l *log.Logger, msg string) {
	l.Log(logContext.SetCustomKeyContext(context.Background()), slog.LevelInfo, msg)
}

func TestSourceOfCustomKeyContextIsWrapperCaller(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBuffer(nil)
	l := log.NewLogger(log.WithLevel(slog.LevelDebug), log.WithOutput(buf))

	_, file, line, _ := runtime.Caller(0)
	logViaWrapper(l, "first")
	logViaWrapper(l, "second")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], fmt.Sprintf(`"source":"log/%s:%d"`, filepath.Base(file), line+1))
	assert.Contains(t, lines[1], fmt.Sprintf(`"source":"log/%s:%d"`, filepath.Base(file), line+2))
}

func TestConcurrentRecordsDoNotInterleave(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 8
		records    = 200
	)

	var first, second bytes.Buffer

	l := log.NewLogger(log.WithLevel(slog.LevelDebug), log.WithOutput(&first))
	children := []*log.Logger{l, l.Named("child"), l.With("k", "v"), l.WithGroup("g")}

	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			child := children[i%len(children)]
			for j := range records {
				child.Info(strings.Repeat("x", j), "i", i, "j", j)
			}
		}()
	}

	// switching the output while records are written must be safe
	l.SetOutput(&second)

	wg.Wait()

	lines := strings.Split(strings.TrimSpace(first.String()+second.String()), "\n")
	require.Len(t, lines, goroutines*records)

	for _, line := range lines {
		require.True(t, json.Valid([]byte(line)), line)
	}
}

func TestNewJSONHandlerUsesLevelAndAddSource(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBuffer(nil)
	h := log.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn, AddSource: true}, func(t time.Time) time.Time { return t })
	l := slog.New(h)

	l.Info("skipped")
	l.Warn("written")

	assert.NotContains(t, buf.String(), "skipped")
	assert.Contains(t, buf.String(), `"msg":"written","source":"log/handler_test.go:`)
}
