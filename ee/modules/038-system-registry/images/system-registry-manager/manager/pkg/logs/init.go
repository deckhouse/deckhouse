/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package logs

import (
	"github.com/bombsimon/logrusr/v2"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

type DefaultFieldFormatter struct {
	DefaultFields map[string]interface{}
	JSONFormatter *logrus.JSONFormatter
}

func (f *DefaultFieldFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	for key, value := range f.DefaultFields {
		entry.Data[key] = value
	}

	data, err := f.JSONFormatter.Format(entry)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func init() {
	initMainLogger()
	initK8sClientLogger()
}

func initMainLogger() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func initK8sClientLogger() {
	logrusLog := logrus.New()
	logrusLog.SetFormatter(&DefaultFieldFormatter{
		DefaultFields: map[string]interface{}{
			"app": "k8sClient",
		},
		JSONFormatter: &logrus.JSONFormatter{},
	})

	logrLogger := logrusr.New(logrusLog)
	klog.SetLoggerWithOptions(logrLogger)
}
