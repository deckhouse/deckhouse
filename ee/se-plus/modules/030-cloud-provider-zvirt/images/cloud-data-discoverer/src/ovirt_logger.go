/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"

	"github.com/deckhouse/deckhouse/pkg/log"
	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v3"
)

func NewOvirtLogger(logger *log.Logger) ovirtclientlog.Logger {
	return &oVirtLogger{
		logger: logger,
	}
}

type oVirtLogger struct {
	logger *log.Logger
}

func (o *oVirtLogger) WithContext(_ context.Context) ovirtclientlog.Logger {
	return o
}

func (o *oVirtLogger) Debugf(format string, args ...interface{}) {
	o.logger.Debugf(format, args...)
}

func (o *oVirtLogger) Infof(format string, args ...interface{}) {
	o.logger.Infof(format, args...)
}

func (o *oVirtLogger) Warningf(format string, args ...interface{}) {
	o.logger.Warnf(format, args...)
}

func (o *oVirtLogger) Errorf(format string, args ...interface{}) {
	o.logger.Errorf(format, args...)
}
