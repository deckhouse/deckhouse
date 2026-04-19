// Copyright 2025 Flant JSC
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
	"sync"
	"time"

	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

var _ slog.Handler = (*SlogTextHandler)(nil)

type SlogTextHandler struct {
	slog.Handler

	// output
	w io.Writer
	// buffer for default slog handler
	b *bytes.Buffer
	m *sync.Mutex

	// aggregate logger name
	name string

	// for testing purpose
	timeFn func(t time.Time) time.Time
}

func NewSlogTextHandler(handler slog.Handler) *SlogTextHandler {
	return &SlogTextHandler{
		Handler: handler,
	}
}

func (h *SlogTextHandler) Handle(ctx context.Context, r slog.Record) error {
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
		Level:   Level(r.Level).String(),
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
		logOutput.Fields = attrs
	}

	b, err := logOutput.Text()
	if err != nil {
		fmt.Printf(`{"error":"bad encode","errmsg":%#v,"logOutput":%#v}`+"\n", err.Error(), logOutput)
		return err
	}

	h.w.Write(b)

	return nil
}

func (h *SlogTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) < 1 {
		return h
	}

	h2 := *h
	h2.Handler = h.Handler.WithAttrs(attrs)

	return &h2
}

func (h *SlogTextHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.Handler = h.Handler.WithGroup(name)

	return &h2
}

func (h *SlogTextHandler) Named(name string) slog.Handler {
	currName := name
	if h.name != "" {
		currName = fmt.Sprintf("%s.%s", h.name, name)
	}

	h2 := *h
	h2.name = currName

	return &h2
}

func (h *SlogTextHandler) SetOutput(w io.Writer) {
	h.w = w
}

func NewTextHandler(out io.Writer, opts *slog.HandlerOptions, timeFn func(t time.Time) time.Time) *SlogTextHandler {
	b := new(bytes.Buffer)

	return &SlogTextHandler{
		Handler: slog.NewJSONHandler(b, opts),
		b:       b,
		m:       &sync.Mutex{},
		w:       out,
		timeFn:  timeFn,
	}
}
