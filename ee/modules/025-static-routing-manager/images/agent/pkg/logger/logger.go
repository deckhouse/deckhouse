/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package logger

import (
	"flag"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

const (
	ErrorLevel   Verbosity = "0"
	WarningLevel Verbosity = "1"
	InfoLevel    Verbosity = "2"
	DebugLevel   Verbosity = "3"
	TraceLevel   Verbosity = "4"
)

const (
	warnLvl = iota + 1
	infoLvl
	debugLvl
	traceLvl
)

type (
	Verbosity string
)

type Logger struct {
	log logr.Logger
}

func NewLogger(level Verbosity) (*Logger, error) {
	klog.InitFlags(nil)
	if err := flag.Set("v", string(level)); err != nil {
		return nil, err
	}
	flag.Parse()

	log := klogr.New().WithCallDepth(1)

	return &Logger{log: log}, nil
}

func (l Logger) GetLogger() logr.Logger {
	return l.log
}

func (l Logger) Error(err error, message string, keysAndValues ...interface{}) {
	l.log.Error(err, fmt.Sprintf("ERROR %s", message), keysAndValues...)
}

func (l Logger) Warning(message string, keysAndValues ...interface{}) {
	l.log.V(warnLvl).Info(fmt.Sprintf("WARNING %s", message), keysAndValues...)
}

func (l Logger) Info(message string, keysAndValues ...interface{}) {
	l.log.V(infoLvl).Info(fmt.Sprintf("INFO %s", message), keysAndValues...)
}

func (l Logger) Debug(message string, keysAndValues ...interface{}) {
	l.log.V(debugLvl).Info(fmt.Sprintf("DEBUG %s", message), keysAndValues...)
}

func (l Logger) Trace(message string, keysAndValues ...interface{}) {
	l.log.V(traceLvl).Info(fmt.Sprintf("TRACE %s", message), keysAndValues...)
}
