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

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

// Extends default slog with new log levels
type WrappedLogger struct {
	*slog.Logger
	opts *slog.HandlerOptions
}

func NewSlogLogger(w io.Writer, opts *slog.HandlerOptions) *WrappedLogger {
	return &WrappedLogger{
		Logger: slog.New(slog.NewJSONHandler(w, opts)),
		opts:   opts,
	}
}

var _ slog.Handler = (*SlogHandler)(nil)

type SlogHandler struct {
	slog.Handler

	w io.Writer
	b *bytes.Buffer
	m *sync.Mutex

	name string

	timeFn func(t time.Time) time.Time
}

func NewSlogHandler(handler slog.Handler) *SlogHandler {
	return &SlogHandler{
		Handler: handler,
	}
}

func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.m.Lock()

	defer func() {
		h.b.Reset()
		h.m.Unlock()
	}()

	var (
		tracePtr *string
	)

	isCustom := logContext.GetCustomKeyContext(ctx)
	if isCustom {
		var pc uintptr
		var pcs [1]uintptr
		// skip [runtime.Callers, this function, this function's caller]
		runtime.Callers(5, pcs[:])
		pc = pcs[0]
		r.PC = pc

		tracePtr = logContext.GetStackTraceContext(ctx)
	}

	if err := h.Handler.Handle(ctx, r); err != nil {
		return err
	}

	attrs := map[string]any{}
	if err := json.Unmarshal(h.b.Bytes(), &attrs); err != nil {
		return err
	}

	logOutput := &LogOutput{
		Level:   strings.ToLower(Level(r.Level).String()),
		Time:    h.timeFn(r.Time).Format(time.RFC3339),
		Message: r.Message,
		Name:    h.name,
	}

	// if logger was traced - remove source
	if tracePtr != nil {
		logOutput.Stacktrace = *tracePtr

		delete(attrs, "source")
	}

	fieldSource, ok := attrs["source"]
	if ok {
		logOutput.Source = fieldSource.(string)

		delete(attrs, "source")
	}

	if len(attrs) > 0 {
		b, err := json.Marshal(attrs)
		if err != nil {
			return err
		}

		// drop { and }
		logOutput.FieldsJSON = b[1 : len(b)-1]
	}

	buf := bytes.NewBuffer([]byte{})
	if err := json.NewEncoder(buf).Encode(logOutput); err != nil {
		fmt.Printf(`{"error":"bad encode","errmsg":%#v,"logOutput":%#v}`+"\n", err.Error(), logOutput)
		return err
	}

	h.w.Write(buf.Bytes())

	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) < 1 {
		return h
	}

	h2 := *h
	h2.Handler = h.Handler.WithAttrs(attrs)

	return &h2
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.Handler = h.Handler.WithGroup(name)

	return &h2
}

func (h *SlogHandler) Named(name string) slog.Handler {
	currName := name
	if h.name != "" {
		currName = fmt.Sprintf("%s.%s", h.name, name)
	}

	h2 := *h
	h2.name = currName

	return &h2
}

func NewHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *SlogHandler {
	b := new(bytes.Buffer)

	return &SlogHandler{
		Handler: slog.NewJSONHandler(b, opts),
		b:       b,
		m:       &sync.Mutex{},
		w:       out,
		timeFn:  timeFn,
	}
}
