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

package log_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/stretchr/testify/assert"
)

func Test_JSON_Logger(t *testing.T) {
	t.Parallel()

	const (
		message  = "stub msg"
		argKey   = "stub_arg"
		argValue = "arg"
	)

	buf := bytes.NewBuffer([]byte{})

	logger := log.NewLogger(
		log.WithLevel(slog.LevelDebug),
		log.WithOutput(buf),
		log.WithHandlerType(log.JSONHandlerType),
		log.WithTimeFunc(func(_ time.Time) time.Time {
			parsedTime, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
			if err != nil {
				assert.NoError(t, err)
			}

			return parsedTime
		}),
	)

	t.Run("log output without error", func(t *testing.T) {
		logger.Debug(message, slog.String(argKey, argValue))
		logger.Info(message, slog.String(argKey, argValue))
		logger.Warn(message, slog.String(argKey, argValue))
		//test fatal
		logger.Log(context.Background(), log.LevelFatal.Level(), message, slog.String(argKey, argValue))

		expectedRegex := `^{"level":"(debug|info|warn|fatal)","msg":"stub msg","source":"log\/logger_test.go:[0-9]+","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$`
		reg := regexp.MustCompile(expectedRegex)

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		assert.Equal(t, 4, len(lines))

		for _, line := range lines {
			assert.Regexp(t, reg, line)
		}
	})

	t.Run("log output with error", func(t *testing.T) {
		buf.Reset()

		logger.Error(message, slog.String(argKey, argValue))

		expectedRegex := `^{"level":"error","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$`
		reg := regexp.MustCompile(expectedRegex)

		line := strings.TrimSpace(buf.String())
		assert.Regexp(t, reg, line)
	})
}

// Test that adapter is working through default import in another package
func Test_JSON_Logger_Unmarshal(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBuffer([]byte{})

	logger := log.NewLogger(
		log.WithLevel(slog.LevelDebug),
		log.WithOutput(buf),
	)

	logger.Debug("test debug message")
	logger.Info("test info message")
	logger.Warn("test warn message")
	logger.Error("test error message")
	logger.Log(context.Background(), log.LevelFatal.Level(), "test fatal message")

	// Catch log lines
	lines := []string{}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	assert.Equal(t, 5, len(lines), "should have 5 log lines")

	for _, line := range lines {
		var record map[string]interface{}
		err := json.Unmarshal([]byte(line), &record)
		assert.NoError(t, err, line, "log line should be a valid JSON")
	}
}

func Test_JSON_LoggerFormat(t *testing.T) {
	t.Parallel()

	const (
		message  = "stub msg"
		argKey   = "stub_arg"
		argValue = "arg"
	)

	defaultLogFn := func(logger *log.Logger) {
		logger.Debug(message, slog.String(argKey, argValue))
		logger.Info(message, slog.String(argKey, argValue))
		logger.Warn(message, slog.String(argKey, argValue))
		logger.Error(message, slog.String(argKey, argValue))
		//test fatal
		logger.Log(context.Background(), log.LevelFatal.Level(), message, slog.String(argKey, argValue))
	}

	logfFn := func(logger *log.Logger) {
		logger.Debugf("stub msg: %s", argValue)
		logger.Infof("stub msg: %s", argValue)
		logger.Warnf("stub msg: %s", argValue)
		logger.Errorf("stub msg: %s", argValue)
		//test fatal
		logger.Logf(context.Background(), log.LevelFatal, "stub msg: %s", argValue)
	}

	type meta struct {
		name    string
		enabled bool
	}

	type fields struct {
		logfn          func(logger *log.Logger)
		mutateLoggerfn func(logger *log.Logger) *log.Logger
	}

	type args struct {
		addSource bool
		level     log.Level
	}

	type wants struct {
		containsRegexp    string
		notContainsRegexp string
	}

	tests := []struct {
		meta   meta
		fields fields
		args   args
		wants  wants
	}{
		{
			meta: meta{
				name:    "logger default options is level info and add source false",
				enabled: true,
			},
			fields: fields{
				logfn:          defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger { return logger },
			},
			args: args{},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger.SetLevel(log.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(debug|info|warn|fatal)","msg":"stub msg","source":"log\/logger_test.go:[1-9][0-9][0-9]","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "*f functions logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: logfFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger.SetLevel(log.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(debug|info|warn|fatal)","msg":"stub msg: arg","source":"log\/logger_test.go:([1-9][0-9]|[1-9][0-9][0-9])","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg: arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "logger with name should have field logger",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.Named("first")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","logger":"first","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","logger":"first","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "logger names should separate by dot",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger = logger.Named("first")
					logger = logger.Named("second")
					return logger.Named("third")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","logger":"first.second.third","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "with group should wrap args",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.WithGroup("module")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","msg":"stub msg","module":{"stub_arg":"arg"},"time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","module":{"stub_arg":"arg"},"stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "raw json arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.With(log.RawJSON("stub log", `{"stub arg":{"nested arg":"some"}}`))
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","stub log":{"stub arg":{"nested arg":"some"}},"stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "raw yaml arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.With(log.RawYAML("stub log", `
stubArg:
  nestedArg: some`))
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(info|warn|fatal)","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","stub log":{"stubArg":{"nestedArg":"some"}},"stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(debug|trace)".*`,
			},
		},
		{
			meta: meta{
				name:    "default logger level change must affect logger which set default",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					log.SetDefault(logger)
					log.SetDefaultLevel(log.LevelError)
					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^{"level":"(fatal)","msg":"stub msg","stub_arg":"arg","time":"2006-01-02T15:04:05Z"}$|` +
					`^{"level":"(error)","msg":"stub msg","stub_arg":"arg","stacktrace":.*,"time":"2006-01-02T15:04:05Z"}$)`,
				notContainsRegexp: `^{"level":"(info|warn|debug|trace)".*`,
			},
		},
	}

	for _, tt := range tests {
		if !tt.meta.enabled {
			continue
		}

		t.Run(tt.meta.name, func(t *testing.T) {
			t.Parallel()

			buf := bytes.NewBuffer([]byte{})

			logger := log.NewLogger(
				log.WithLevel(tt.args.level.Level()),
				log.WithOutput(buf),
				log.WithHandlerType(log.JSONHandlerType),
				log.WithTimeFunc(func(_ time.Time) time.Time {
					parsedTime, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
					if err != nil {
						assert.NoError(t, err)
					}

					return parsedTime
				}),
			)

			logger = tt.fields.mutateLoggerfn(logger)

			tt.fields.logfn(logger)

			reg := regexp.MustCompile(tt.wants.containsRegexp)
			ncreg := regexp.MustCompile(tt.wants.notContainsRegexp)

			for _, line := range strings.Split(buf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				assert.Regexp(t, reg, line)
			}

			for _, line := range strings.Split(buf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				assert.NotRegexp(t, ncreg, line)
			}
		})
	}
}

func Test_Text_LoggerFormat(t *testing.T) {
	t.Parallel()

	const (
		message  = "stub msg"
		argKey   = "stub_arg"
		argValue = "arg"
	)

	defaultLogFn := func(logger *log.Logger) {
		logger.Debug(message, slog.String(argKey, argValue))
		logger.Info(message, slog.String(argKey, argValue))
		logger.Warn(message, slog.String(argKey, argValue))
		logger.Error(message, slog.String(argKey, argValue))
		//test fatal
		logger.Log(context.Background(), log.LevelFatal.Level(), message, slog.String(argKey, argValue))
	}

	logfFn := func(logger *log.Logger) {
		logger.Debugf("stub msg: %s", argValue)
		logger.Infof("stub msg: %s", argValue)
		logger.Warnf("stub msg: %s", argValue)
		logger.Errorf("stub msg: %s", argValue)
		//test fatal
		logger.Logf(context.Background(), log.LevelFatal, "stub msg: %s", argValue)
	}

	type meta struct {
		name    string
		enabled bool
	}

	type fields struct {
		logfn          func(logger *log.Logger)
		mutateLoggerfn func(logger *log.Logger) *log.Logger
	}

	type args struct {
		addSource bool
		level     log.Level
	}

	type wants struct {
		containsRegexp    string
		notContainsRegexp string
	}

	tests := []struct {
		meta   meta
		fields fields
		args   args
		wants  wants
	}{
		{
			meta: meta{
				name:    "logger default options is level info and add source false",
				enabled: true,
			},
			fields: fields{
				logfn:          defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger { return logger },
			},
			args: args{},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) msg='stub msg' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger.SetLevel(log.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (DEBUG|INFO|WARN|FATAL) msg='stub msg' source=log\/logger_test.go:[1-9][0-9][0-9] stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (TRACE)".*`,
			},
		},
		{
			meta: meta{
				name:    "*f functions logger change to debug level should contains addsource and debug level",
				enabled: true,
			},
			fields: fields{
				logfn: logfFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger.SetLevel(log.LevelDebug)

					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (DEBUG|INFO|WARN|FATAL) msg='stub msg: arg' source=log\/logger_test.go:([1-9][0-9]|[1-9][0-9][0-9])$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg: arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "logger with name should have field logger",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.Named("first")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) logger=first msg='stub msg' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) logger=first msg='stub msg' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "logger names should separate by dot",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					logger = logger.Named("first")
					logger = logger.Named("second")
					return logger.Named("third")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) logger=first.second.third msg='stub msg' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) logger=first.second.third msg='stub msg' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "with group should wrap args",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.WithGroup("module")
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) msg='stub msg' module.stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' module.stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "raw json arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.With(log.RawJSON("stub log", `{"stub arg":{"nested arg":"some"}}`))
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) msg='stub msg' stub log.stub arg.nested arg='some' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' stub log.stub arg.nested arg='some' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "raw yaml arg should be formatted like structure",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					return logger.With(log.RawYAML("stub log", `
stubArg:
  nestedArg: some`))
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (INFO|WARN|FATAL) msg='stub msg' stub log.stubArg.nestedArg='some' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' stub log.stubArg.nestedArg='some' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (DEBUG|TRACE).*`,
			},
		},
		{
			meta: meta{
				name:    "default logger level change must affect logger which set default",
				enabled: true,
			},
			fields: fields{
				logfn: defaultLogFn,
				mutateLoggerfn: func(logger *log.Logger) *log.Logger {
					log.SetDefault(logger)
					log.SetDefaultLevel(log.LevelError)
					return logger
				},
			},
			args: args{
				addSource: false,
				level:     log.LevelInfo,
			},
			wants: wants{
				containsRegexp: `(^2006-01-02T15:04:05Z (FATAL) msg='stub msg' stub_arg='arg'$|` +
					`^2006-01-02T15:04:05Z (ERROR) msg='stub msg' stub_arg='arg' stacktrace=.*$)`,
				notContainsRegexp: `^2006-01-02T15:04:05Z (INFO|WARN|DEBUG|TRACE).*`,
			},
		},
	}

	for _, tt := range tests {
		if !tt.meta.enabled {
			continue
		}

		t.Run(tt.meta.name, func(t *testing.T) {
			t.Parallel()

			buf := bytes.NewBuffer([]byte{})

			logger := log.NewLogger(
				log.WithLevel(tt.args.level.Level()),
				log.WithOutput(buf),
				log.WithHandlerType(log.TextHandlerType),
				log.WithTimeFunc(func(_ time.Time) time.Time {
					parsedTime, err := time.Parse(time.DateTime, "2006-01-02 15:04:05")
					if err != nil {
						assert.NoError(t, err)
					}

					return parsedTime
				}),
			)

			logger = tt.fields.mutateLoggerfn(logger)

			tt.fields.logfn(logger)

			reg := regexp.MustCompile(tt.wants.containsRegexp)
			ncreg := regexp.MustCompile(tt.wants.notContainsRegexp)

			for _, line := range strings.Split(buf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				assert.Regexp(t, reg, line)
			}

			for _, line := range strings.Split(buf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				assert.NotRegexp(t, ncreg, line)
			}
		})
	}
}
