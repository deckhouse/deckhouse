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
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	logContext "github.com/deckhouse/deckhouse/pkg/log/context"
)

// compatWant holds what v0.2.1 wrote for compatCases. That version encoded every record with
// slog.JSONHandler, decoded it into a map and encoded the map again; the one-pass handler
// must keep the output byte for byte.
func TestOutputCompatibility(t *testing.T) {
	t.Parallel()

	for _, tc := range compatCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want, ok := compatWant[tc.name]
			require.True(t, ok, "no expected output")

			assert.Equal(t, want.json, runCompatCase(tc, log.JSONHandlerType))
			assert.Equal(t, want.text, runCompatCase(tc, log.TextHandlerType))
		})
	}
}

type compatCase struct {
	name string
	run  func(l *log.Logger)
}

type compatOutput struct {
	json []string
	text []string
}

var compatTime = time.Date(2006, 1, 2, 15, 4, 5, 123456789, time.FixedZone("MSK", 3*3600))

func runCompatCase(tc compatCase, handlerType log.HandlerType) []string {
	buf := bytes.NewBuffer(nil)

	tc.run(log.NewLogger(
		log.WithLevel(slog.LevelInfo),
		log.WithOutput(buf),
		log.WithHandlerType(handlerType),
		log.WithTimeFunc(func(time.Time) time.Time { return compatTime }),
	))

	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

var compatStrings = []string{
	"plain",
	"",
	`quote " and backslash \ and slash /`,
	"line1\nline2\r\ttab",
	"<script>alert('x') & </script>",
	"кириллица и emoji 🚀",
	"sep line para",
	"ctl \x1b[31mred\x1b[0m \x00 \x07 \x0b \x7f",
	"bs\bff\f",
	"invalid \xff\xfe utf8 \xe2\x80",
	"single ' quote",
}

type compatStruct struct {
	Zeta  string            `json:"zeta"`
	Alpha int               `json:"alpha"`
	Mid   map[string]any    `json:"mid,omitempty"`
	Empty string            `json:"empty,omitempty"`
	HTML  string            `json:"html"`
	Inner *compatStruct     `json:"inner,omitempty"`
	Tags  map[string]string `json:"tags"`
	Float float64           `json:"float"`
	Big   int64             `json:"big"`
}

type compatMarshalError struct{ msg string }

func (e compatMarshalError) Error() string { return e.msg }

func (e compatMarshalError) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"kind": "marshal", "msg": %q}`, e.msg)), nil
}

type compatValuer struct{ v slog.Value }

func (v compatValuer) LogValue() slog.Value { return v.v }

type compatStringer struct {
	A int
	b int
}

func (s compatStringer) String() string { return fmt.Sprintf("stringer %d", s.b) }

var compatCases = []compatCase{
	{"simple", func(l *log.Logger) {
		l.Info("hello")
	}},
	{"message escaping", func(l *log.Logger) {
		for _, s := range compatStrings {
			l.Info(s)
		}
	}},
	{"value escaping", func(l *log.Logger) {
		for _, s := range compatStrings {
			l.Info("m", "k", s)
		}
	}},
	{"key escaping", func(l *log.Logger) {
		for _, s := range compatStrings {
			l.Info("m", s+"k", "v")
		}
	}},
	{"logger name escaping", func(l *log.Logger) {
		for _, s := range compatStrings {
			l.Named(s + "n").Info("m")
		}
	}},
	{"numbers", func(l *log.Logger) {
		l.Info("m",
			"i0", 0, "ineg", -1, "i42", 42, "i53", int64(1)<<53, "ineg53", -(int64(1) << 53),
			"u0", uint64(0), "u53", uint64(1)<<53,
			"f0", 0.0, "fneg0", math.Copysign(0, -1), "f15", 1.5, "f1e7", 1e-7, "f1e6", 1e-6,
			"fmid", 123456789.125, "f1e20", 1e20, "f1e21", 1e21, "fmax", math.MaxFloat64, "fmin", 5e-324,
			"f32", float32(0.1),
			"bt", true, "bf", false,
			"dur", 1500*time.Millisecond, "durneg", -time.Second,
			"tutc", time.Date(2024, 5, 6, 7, 8, 9, 100, time.UTC), "tzone", compatTime,
		)
	}},
	{"non-finite floats", func(l *log.Logger) {
		l.Info("m", "nan", math.NaN(), "pinf", math.Inf(1), "ninf", math.Inf(-1))
	}},
	{"any values", func(l *log.Logger) {
		n := 7
		err := errors.New("boom <&>")

		l.Info("m",
			slog.Any("nil", nil),
			slog.Any("err", err),
			log.Err(err),
			slog.Any("marshal error", compatMarshalError{"bad <thing>"}),
			slog.Any("struct", compatStruct{
				Zeta: "z", Alpha: 1, HTML: "<b>&</b>", Float: 1e21, Big: 1 << 40,
				Mid:   map[string]any{"b": 1, "a": []any{1, "x", nil}},
				Tags:  map[string]string{"y": "1", "x": "<2>"},
				Inner: &compatStruct{Zeta: "inner"},
			}),
			slog.Any("ptr", &n),
			slog.Any("map", map[string]int{"b": 2, "a": 1}),
			slog.Any("int keys", map[int]string{10: "ten", 2: "two"}),
			slog.Any("strings", []string{"<a>", "b", ""}),
			slog.Any("bytes", []byte("hi <there>")),
			slog.Any("raw message", json.RawMessage(`{"b": 1, "a": [1, 2], "c": "<x>"}`)),
			slog.Any("anys", []any{1, "two", 3.5, nil, map[string]any{"z": 1, "a": 2}}),
			slog.Any("empty map", map[string]any{}),
			slog.Any("empty slice", []string{}),
			slog.Any("nil slice", []string(nil)),
			slog.Any("stringer", compatStringer{A: 1}),
			slog.Any("chan", make(chan int)),
			slog.Any("nan in map", map[string]float64{"x": math.NaN()}),
			slog.Any("int", 5),
			slog.Any("time", compatTime),
			slog.Any("duration", time.Minute),
		)
	}},
	{"groups", func(l *log.Logger) {
		l.Info("m",
			slog.Group("g", "b", 1, "a", "x", slog.Group("inner", "z", true, "y", nil)),
			slog.Group("empty"),
			slog.Group("", "inl1", 1, "inl2", 2),
			slog.Group("with source", "source", "s", "ok", 1),
			"after", 1,
		)
		l.Info("m", slog.Group("dropped", "msg", "x", "level", 1, "time", 2))
		l.Info("m", "a", 1, slog.Group("dropped", "msg", "x"))
	}},
	{"repeated keys", func(l *log.Logger) {
		l.With("k", 1, "k", 2).Info("m", "k", 3, "j", 1, "j", 2)
		l.With("g", 1).WithGroup("g").Info("m", "a", 1)
		l.WithGroup("g").Info("m", slog.Group("x", "y", 1), "x", 5)
		l.Info("m", slog.Group("x", "y", 1), "x", 5, slog.Group("x", "z", 2))
	}},
	{"reserved keys", func(l *log.Logger) {
		l.Info("m", "msg", "x", "level", "y", "time", "z", "source", "s", "_source", "u", "logger", "l", "stacktrace", "st", "", "empty key")
		l.Named("name").Info("m", "logger", "l")
	}},
	{"logger chains", func(l *log.Logger) {
		l.With("a", 1).WithGroup("g").With("b", 2).WithGroup("h").Info("m", "c", 3)
		l.WithGroup("g").Info("m")
		l.WithGroup("g").With("a", 1).WithGroup("h").Info("m")
		l.WithGroup("g").WithGroup("h").With("a", 1).Info("m")
		l.WithGroup("g").Info("m", "msg", "dropped")
		l.WithGroup("g").With("a", 1).Info("m", "msg", "dropped")
		l.With("z", 1).WithGroup("g").With("y", 2).Info("m", "x", 3)
		l.With("w", 1).Named("first").Named("second").WithGroup("grp").Info("m", "k", "v")
	}},
	{"sorted keys", func(l *log.Logger) {
		// shell-operator builds hook loggers like this and compares the lines exactly
		l.With("a", "b").With("output", "stdout").With("proxyJsonLog", true).With(slog.Any("hook_foo", "baz")).Info("hook result")
		l.With("z", 1, "m", 2).Info("m", "z1", 1, "a", 2, "M", 3, "_", 4, "a.b", 5, "a b", 6, "Z", 7, "a_", 8)
	}},
	{"log valuers", func(l *log.Logger) {
		l.Info("m",
			slog.Any("string", compatValuer{slog.StringValue("resolved <x>")}),
			slog.Any("group", compatValuer{slog.GroupValue(slog.Int("b", 1), slog.String("a", "x"))}),
			slog.Any("nested", compatValuer{slog.AnyValue(compatValuer{slog.IntValue(9)})}),
		)
	}},
	{"raw json and yaml", func(l *log.Logger) {
		l.Info("m",
			log.RawJSON("json", `{"b": {"y": 1, "x": [1, 2.5, "s", null, true]}, "a": "<html>", "big": 12345678901234567890}`),
			log.RawJSON("bad json", `{"b": `),
			log.RawJSON("scalar json", `"just a string"`),
			log.RawYAML("yaml", "b: 1\na:\n  - x\n  - 2\n  - 3.5\nt: 2024-01-02T03:04:05Z\nnested:\n  z: true\n  y: ~\n"),
			log.RawYAML("bad yaml", "a: b: c"),
		)
		l.With(log.RawJSON("cfg", `{"z": 1, "a": {"c": 2, "b": 3}}`)).Info("m", "k", "v")
	}},
	{"levels", func(l *log.Logger) {
		for _, level := range []slog.Level{0, 1, 4, 5, 8, 10, 12, 16} {
			l.Log(context.Background(), level, "level")
		}
	}},
	{"stacktrace from context", func(l *log.Logger) {
		ctx := logContext.SetCustomKeyContext(context.Background())

		l.Log(logContext.SetStackTraceContext(ctx, `[ {"Func": "main.<lambda> & co", "Line": 1} ]`), slog.LevelError, "json stack", "k", "v")
		l.Log(logContext.SetStackTraceContext(ctx, "plain <stack> & \"text\"\n\tat line"), slog.LevelError, "text stack")
		l.Log(logContext.SetStackTraceContext(ctx, ""), slog.LevelError, "empty stack")
		l.Log(ctx, slog.LevelWarn, "custom key without stack")
	}},
}

var compatWant = map[string]compatOutput{
	"simple": {
		json: []string{
			`{"level":"info","msg":"hello","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='hello' `,
		},
	},
	"message escaping": {
		json: []string{
			`{"level":"info","msg":"plain","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"quote \" and backslash \\ and slash /","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"line1\nline2\r\ttab","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"\u003cscript\u003ealert('x') \u0026 \u003c/script\u003e","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"кириллица и emoji 🚀","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"sep\u2028line\u2029para","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"msg\":\"ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7f\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","msg":"bs\u0008ff\u000c","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"msg\":\"invalid \xff\xfe utf8 \xe2\x80\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","msg":"single ' quote","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='plain' `,
			`2006-01-02T15:04:05+03:00 INFO msg='' `,
			`2006-01-02T15:04:05+03:00 INFO msg='quote \" and backslash \\ and slash /' `,
			`2006-01-02T15:04:05+03:00 INFO msg='line1\nline2\r\ttab' `,
			`2006-01-02T15:04:05+03:00 INFO msg='\u003cscript>alert('x') & \u003c/script>' `,
			`2006-01-02T15:04:05+03:00 INFO msg='кириллица и emoji 🚀' `,
			`2006-01-02T15:04:05+03:00 INFO msg='sep line para' `,
			"2006-01-02T15:04:05+03:00 INFO msg='ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7f' ",
			`2006-01-02T15:04:05+03:00 INFO msg='bs\u0008ff\u000c' `,
			"2006-01-02T15:04:05+03:00 INFO msg='invalid \xff\xfe utf8 \xe2\x80' ",
			`2006-01-02T15:04:05+03:00 INFO msg='single ' quote' `,
		},
	},
	"value escaping": {
		json: []string{
			`{"level":"info","msg":"m","k":"plain","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"quote \" and backslash \\ and slash /","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"line1\nline2\r\ttab","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"\u003cscript\u003ealert('x') \u0026 \u003c/script\u003e","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"кириллица и emoji 🚀","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"sep\u2028line\u2029para","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"msg\":\"m\",\"k\":\"ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7f\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","msg":"m","k":"bs\bff\f","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"invalid �� utf8 ��","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"single ' quote","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='plain' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='quote \" and backslash \\ and slash /' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='line1\nline2\r\ttab' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='\u003cscript>alert('x') & \u003c/script>' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='кириллица и emoji 🚀' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='sep line para' `,
			"2006-01-02T15:04:05+03:00 INFO msg='m' k='ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7f' ",
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='bs\u0008ff\u000c' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='invalid �� utf8 ��' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='single ' quote' `,
		},
	},
	"key escaping": {
		json: []string{
			`{"level":"info","msg":"m","plaink":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","k":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","quote \" and backslash \\ and slash /k":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","line1\nline2\r\ttabk":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","\u003cscript\u003ealert('x') \u0026 \u003c/script\u003ek":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","кириллица и emoji 🚀k":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","sep\u2028line\u2029parak":"v","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"msg\":\"m\",\"ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7fk\":\"v\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","msg":"m","bs\bff\fk":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","invalid �� utf8 ��k":"v","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","single ' quotek":"v","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' plaink='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' k='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' quote \" and backslash \\ and slash /k='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' line1\nline2\r\ttabk='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' \u003cscript>alert('x') & \u003c/script>k='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' кириллица и emoji 🚀k='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' sep line parak='v' `,
			"2006-01-02T15:04:05+03:00 INFO msg='m' ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7fk='v' ",
			`2006-01-02T15:04:05+03:00 INFO msg='m' bs\u0008ff\u000ck='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' invalid �� utf8 ��k='v' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' single ' quotek='v' `,
		},
	},
	"logger name escaping": {
		json: []string{
			`{"level":"info","logger":"plainn","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"n","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"quote \" and backslash \\ and slash /n","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"line1\nline2\r\ttabn","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"\u003cscript\u003ealert('x') \u0026 \u003c/script\u003en","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"кириллица и emoji 🚀n","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"sep\u2028line\u2029paran","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"logger\":\"ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7fn\",\"msg\":\"m\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","logger":"bs\u0008ff\u000cn","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			"{\"level\":\"info\",\"logger\":\"invalid \xff\xfe utf8 \xe2\x80n\",\"msg\":\"m\",\"time\":\"2006-01-02T15:04:05+03:00\"}",
			`{"level":"info","logger":"single ' quoten","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO logger=plainn msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=n msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=quote \" and backslash \\ and slash /n msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=line1\nline2\r\ttabn msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=\u003cscript>alert('x') & \u003c/script>n msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=кириллица и emoji 🚀n msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO logger=sep line paran msg='m' `,
			"2006-01-02T15:04:05+03:00 INFO logger=ctl \\u001b[31mred\\u001b[0m \\u0000 \\u0007 \\u000b \x7fn msg='m' ",
			`2006-01-02T15:04:05+03:00 INFO logger=bs\u0008ff\u000cn msg='m' `,
			"2006-01-02T15:04:05+03:00 INFO logger=invalid \xff\xfe utf8 \xe2\x80n msg='m' ",
			`2006-01-02T15:04:05+03:00 INFO logger=single ' quoten msg='m' `,
		},
	},
	"numbers": {
		json: []string{
			`{"level":"info","msg":"m","bf":false,"bt":true,"dur":1500000000,"durneg":-1000000000,"f0":0,"f15":1.5,"f1e20":100000000000000000000,"f1e21":1e+21,"f1e6":0.000001,"f1e7":1e-7,"f32":0.10000000149011612,"fmax":1.7976931348623157e+308,"fmid":123456789.125,"fmin":5e-324,"fneg0":-0,"i0":0,"i42":42,"i53":9007199254740992,"ineg":-1,"ineg53":-9007199254740992,"tutc":"2024-05-06T07:08:09.0000001Z","tzone":"2006-01-02T15:04:05.123456789+03:00","u0":0,"u53":9007199254740992,"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' bf='false' bt='true' dur='1500000000' durneg='-1000000000' f0='0' f15='1.5' f1e20='100000000000000000000' f1e21='1000000000000000000000' f1e6='0.000001' f1e7='0.0000001' f32='0.10000000149011612' fmax='179769313486231570000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000' fmid='123456789.125' fmin='0.000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000005' fneg0='-0' i0='0' i42='42' i53='9007199254740992' ineg='-1' ineg53='-9007199254740992' tutc='2024-05-06T07:08:09.0000001Z' tzone='2006-01-02T15:04:05.123456789+03:00' u0='0' u53='9007199254740992' `,
		},
	},
	"non-finite floats": {
		json: []string{
			`{"level":"info","msg":"m","nan":"!ERROR:json: unsupported value: NaN","ninf":"!ERROR:json: unsupported value: -Inf","pinf":"!ERROR:json: unsupported value: +Inf","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' nan='!ERROR:json: unsupported value: NaN' ninf='!ERROR:json: unsupported value: -Inf' pinf='!ERROR:json: unsupported value: +Inf' `,
		},
	},
	"any values": {
		json: []string{
			`{"level":"info","msg":"m","anys":[1,"two",3.5,null,{"a":2,"z":1}],"bytes":"aGkgPHRoZXJlPg==","chan":"!ERROR:json: unsupported type: chan int","duration":60000000000,"empty map":{},"empty slice":[],"err":"boom \u003c\u0026\u003e","error":"boom \u003c\u0026\u003e","int":5,"int keys":{"10":"ten","2":"two"},"map":{"a":1,"b":2},"marshal error":{"kind":"marshal","msg":"bad \u003cthing\u003e"},"nan in map":"!ERROR:json: unsupported value: NaN","nil":null,"nil slice":null,"ptr":7,"raw message":{"a":[1,2],"b":1,"c":"\u003cx\u003e"},"stringer":{"A":1},"strings":["\u003ca\u003e","b",""],"struct":{"alpha":1,"big":1099511627776,"float":1e+21,"html":"\u003cb\u003e\u0026\u003c/b\u003e","inner":{"alpha":0,"big":0,"float":0,"html":"","tags":null,"zeta":"inner"},"mid":{"a":[1,"x",null],"b":1},"tags":{"x":"\u003c2\u003e","y":"1"},"zeta":"z"},"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' anys[0]='1' anys[1]='two' anys[2]='3.5' !SOMETHING GOES WRONG. type: <nil> anys[4].a='2' anys[4].z='1' bytes='aGkgPHRoZXJlPg==' chan='!ERROR:json: unsupported type: chan int' duration='60000000000'   err='boom \u003c&>' error='boom \u003c&>' int='5' int keys.10='ten' int keys.2='two' map.a='1' map.b='2' marshal error.kind='marshal' marshal error.msg='bad \u003cthing>' nan in map='!ERROR:json: unsupported value: NaN' !SOMETHING GOES WRONG. type: <nil> !SOMETHING GOES WRONG. type: <nil> ptr='7' raw message.a[0]='1' raw message.a[1]='2' raw message.b='1' raw message.c='\u003cx>' stringer.A='1' strings[0]='\u003ca>' strings[1]='b' strings[2]='' struct.alpha='1' struct.big='1099511627776' struct.float='1000000000000000000000' struct.html='\u003cb>&\u003c/b>' struct.inner.alpha='0' struct.inner.big='0' struct.inner.float='0' struct.inner.html='' !SOMETHING GOES WRONG. type: <nil> struct.inner.zeta='inner' struct.mid.a[0]='1' struct.mid.a[1]='x' !SOMETHING GOES WRONG. type: <nil> struct.mid.b='1' struct.tags.x='\u003c2>' struct.tags.y='1' struct.zeta='z' `,
		},
	},
	"groups": {
		json: []string{
			`{"level":"info","msg":"m","after":1,"g":{"a":"x","b":1,"inner":{"y":null,"z":true}},"inl1":1,"inl2":2,"with source":{"ok":1},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","a":1,"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' after='1' g.a='x' g.b='1' !SOMETHING GOES WRONG. type: <nil> g.inner.z='true' inl1='1' inl2='2' with source.ok='1' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' a='1' `,
		},
	},
	"repeated keys": {
		json: []string{
			`{"level":"info","msg":"m","j":2,"k":3,"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"a":1},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"x":5},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","x":{"z":2},"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' j='2' k='3' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.a='1' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.x='5' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' x.z='2' `,
		},
	},
	"reserved keys": {
		json: []string{
			`{"level":"info","msg":"m","":"empty key","_source":"u","logger":"l","stacktrace":"st","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"name","msg":"m","logger":"l","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' ='empty key' _source='u' logger='l' stacktrace='st' `,
			`2006-01-02T15:04:05+03:00 INFO logger=name msg='m' logger='l' `,
		},
	},
	"logger chains": {
		json: []string{
			`{"level":"info","msg":"m","a":1,"g":{"b":2,"h":{"c":3}},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"a":1},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"h":{"a":1}},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"a":1},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","g":{"x":3,"y":2},"z":1,"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","logger":"first.second","msg":"m","grp":{"k":"v"},"w":1,"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' a='1' g.b='2' g.h.c='3' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.a='1' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.h.a='1' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.a='1' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' g.x='3' g.y='2' z='1' `,
			`2006-01-02T15:04:05+03:00 INFO logger=first.second msg='m' grp.k='v' w='1' `,
		},
	},
	"sorted keys": {
		json: []string{
			`{"level":"info","msg":"hook result","a":"b","hook_foo":"baz","output":"stdout","proxyJsonLog":true,"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","M":3,"Z":7,"_":4,"a":2,"a b":6,"a.b":5,"a_":8,"m":2,"z":1,"z1":1,"time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='hook result' a='b' hook_foo='baz' output='stdout' proxyJsonLog='true' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' M='3' Z='7' _='4' a='2' a b='6' a.b='5' a_='8' m='2' z='1' z1='1' `,
		},
	},
	"log valuers": {
		json: []string{
			`{"level":"info","msg":"m","group":{"a":"x","b":1},"nested":9,"string":"resolved \u003cx\u003e","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' group.a='x' group.b='1' nested='9' string='resolved \u003cx>' `,
		},
	},
	"raw json and yaml": {
		json: []string{
			`{"level":"info","msg":"m","bad json":"{\"b\": ","bad yaml":"a: b: c","json":{"a":"\u003chtml\u003e","b":{"x":[1,2.5,"s",null,true],"y":1},"big":12345678901234567000},"scalar json":"\"just a string\"","yaml":{"a":["x",2,3.5],"b":1,"nested":{"y":null,"z":true},"t":"2024-01-02T03:04:05Z"},"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info","msg":"m","cfg":{"a":{"b":3,"c":2},"z":1},"k":"v","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='m' bad json='{\"b\": ' bad yaml='a: b: c' json.a='\u003chtml>' json.b.x[0]='1' json.b.x[1]='2.5' json.b.x[2]='s' !SOMETHING GOES WRONG. type: <nil> json.b.x[4]='true' json.b.y='1' json.big='12345678901234567000' scalar json='\"just a string\"' yaml.a[0]='x' yaml.a[1]='2' yaml.a[2]='3.5' yaml.b='1' !SOMETHING GOES WRONG. type: <nil> yaml.nested.z='true' yaml.t='2024-01-02T03:04:05Z' `,
			`2006-01-02T15:04:05+03:00 INFO msg='m' cfg.a.b='3' cfg.a.c='2' cfg.z='1' k='v' `,
		},
	},
	"levels": {
		json: []string{
			`{"level":"info","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"info+1","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"warn","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"warn+1","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"error","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"error+2","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"fatal","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"fatal+4","msg":"level","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 INFO msg='level' `,
			`2006-01-02T15:04:05+03:00 INFO+1 msg='level' `,
			`2006-01-02T15:04:05+03:00 WARN msg='level' `,
			`2006-01-02T15:04:05+03:00 WARN+1 msg='level' `,
			`2006-01-02T15:04:05+03:00 ERROR msg='level' `,
			`2006-01-02T15:04:05+03:00 ERROR+2 msg='level' `,
			`2006-01-02T15:04:05+03:00 FATAL msg='level' `,
			`2006-01-02T15:04:05+03:00 FATAL+4 msg='level' `,
		},
	},
	"stacktrace from context": {
		json: []string{
			`{"level":"error","msg":"json stack","k":"v","stacktrace":[{"Func":"main.\u003clambda\u003e \u0026 co","Line":1}],"time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"error","msg":"text stack","stacktrace":"plain \u003cstack\u003e \u0026 \"text\"\n\tat line","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"error","msg":"empty stack","time":"2006-01-02T15:04:05+03:00"}`,
			`{"level":"warn","msg":"custom key without stack","time":"2006-01-02T15:04:05+03:00"}`,
		},
		text: []string{
			`2006-01-02T15:04:05+03:00 ERROR msg='json stack' k='v' stacktrace=[ {\"Func\": \"main.\u003clambda> & co\", \"Line\": 1} ]`,
			`2006-01-02T15:04:05+03:00 ERROR msg='text stack' stacktrace=plain \u003cstack> & \"text\"\n\tat line`,
			`2006-01-02T15:04:05+03:00 ERROR msg='empty stack' `,
			`2006-01-02T15:04:05+03:00 WARN msg='custom key without stack' `,
		},
	},
}
