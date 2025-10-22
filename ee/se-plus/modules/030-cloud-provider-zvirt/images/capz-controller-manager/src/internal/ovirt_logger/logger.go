/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ovirt_logger

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v3"
	"k8s.io/klog/v2/textlogger"
)

type KLogr struct {
	logger logr.Logger

	VDebug   int
	VInfo    int
	VWarning int
}

func (g *KLogr) Debugf(format string, args ...interface{}) {
	g.logger.V(g.VDebug).Info(fmt.Sprintf(format, args...))
}

func (g *KLogr) Infof(format string, args ...interface{}) {
	g.logger.V(g.VInfo).Info(fmt.Sprintf(format, args...))
}

func (g *KLogr) Warningf(format string, args ...interface{}) {
	g.logger.V(g.VWarning).Info(fmt.Sprintf(format, args...))
}

func (g *KLogr) Errorf(format string, args ...interface{}) {
	g.logger.Error(fmt.Errorf(format, args...), "error")
}

func (g *KLogr) WithContext(_ context.Context) ovirtclientlog.Logger {
	return g
}

func (g *KLogr) WithVDebug(level int) *KLogr {
	g.VDebug = level
	return g
}

func (g *KLogr) WithVInfo(level int) *KLogr {
	g.VInfo = level
	return g
}

func (g *KLogr) WithVWarning(level int) *KLogr {
	g.VWarning = level
	return g
}

func NewKLogr(names ...string) *KLogr {
	logger := textlogger.NewLogger(textlogger.NewConfig()).WithCallDepth(1)
	for _, name := range names {
		logger = logger.WithName(name)
	}

	return &KLogr{
		logger:   logger,
		VDebug:   5,
		VInfo:    0,
		VWarning: 0,
	}
}
