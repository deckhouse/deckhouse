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

package app

import (
	log "github.com/sirupsen/logrus"
)

const (
	loggerSimple = "simple"
	loggerJSON   = "json"
)

func InitLogger() *log.Entry {
	var formatter log.Formatter = &log.TextFormatter{
		DisableColors:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}
	if LoggerType == loggerJSON {
		formatter = &log.JSONFormatter{}
	}

	l := log.New()
	l.SetLevel(log.Level(LoggerLevel))
	l.SetFormatter(formatter)

	return log.NewEntry(l)
}
