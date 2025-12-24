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

import "github.com/name212/govalue"

type LoggerProvider func() Logger

func SimpleLoggerProvider(logger Logger) LoggerProvider {
	return func() Logger {
		return logger
	}
}

func GetDefaultLoggerProvider() LoggerProvider {
	return DefaultLoggerProvider
}

func DefaultLoggerProvider() Logger {
	return GetDefaultLogger()
}

func SafeProvideLogger(provider LoggerProvider) Logger {
	// GetDefaultLogger does not create new logger use pre-created
	return provideSafe(provider, GetDefaultLogger())
}

var silentLoggerInstance = NewSilentLogger()

func SafeProvideLoggerOrSilent(provider LoggerProvider) Logger {
	return provideSafe(provider, silentLoggerInstance)
}

func provideSafe(provider LoggerProvider, defaultLogger Logger) Logger {
	if provider != nil {
		logger := provider()
		if !govalue.IsNil(logger) {
			return logger
		}
	}

	return defaultLogger
}
