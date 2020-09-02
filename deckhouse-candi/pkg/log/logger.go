package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/flant/logboek"
	"github.com/sirupsen/logrus"

	"flant/deckhouse-candi/pkg/app"
)

var defaultLogger Logger

func InitLogger(loggerType string) {
	switch loggerType {
	case "pretty":
		defaultLogger = NewPrettyLogger()
	case "simple":
		defaultLogger = NewSimpleLogger()
	default:
		panic("unknown logger type: " + app.LoggerType)
	}
}

type Logger interface {
	LogProcess(string, string, func() error) error

	LogInfoF(format string, a ...interface{})
	LogInfoLn(a ...interface{})
	LogErrorF(format string, a ...interface{})
	LogErrorLn(a ...interface{})

	LogSuccess(string)
	LogWarnLn(string)
	LogFail(string)

	LogJSON([]byte)
}

var (
	_ Logger = &PrettyLogger{}
	_ Logger = &SimpleLogger{}
)

type styleEntry struct {
	title   string
	options logboek.LogProcessOptions
}

type PrettyLogger struct {
	processTitles map[string]styleEntry
}

func NewPrettyLogger() *PrettyLogger {
	err := logboek.Init()
	if err != nil {
		panic(fmt.Errorf("can't start logging system: %w", err))
	}
	logboek.SetLevel(logboek.Info)
	logboek.SetWidth(logboek.DefaultWidth)

	return &PrettyLogger{
		processTitles: map[string]styleEntry{
			"common":    {"\U0001FA81 ~ Common: %s", CommonOptions()},
			"terraform": {"🌱 ~ Terraform: %s", TerraformOptions()},
			"converge":  {"🛸 ~ Converge: %s", ConvergeOptions()},
			"bootstrap": {"⛵ ~ Bootstrap: %s", BootstrapOptions()},
			"default":   {"%s", BoldOptions()},
		},
	}
}

func (d *PrettyLogger) LogProcess(p string, t string, run func() error) error {
	format, ok := d.processTitles[p]
	if !ok {
		format = d.processTitles["default"]
	}
	return logboek.LogProcess(fmt.Sprintf(format.title, t), format.options, run)
}

func (d *PrettyLogger) LogInfoF(format string, a ...interface{}) {
	logboek.LogInfoF(format, a...)
}

func (d *PrettyLogger) LogInfoLn(a ...interface{}) {
	logboek.LogInfoLn(a...)
}

func (d *PrettyLogger) LogErrorF(format string, a ...interface{}) {
	logboek.LogErrorF(format, a...)
}

func (d *PrettyLogger) LogErrorLn(a ...interface{}) {
	logboek.LogErrorLn(a...)
}

func (d *PrettyLogger) LogSuccess(l string) {
	d.LogInfoF("✅ %s", l)
}

func (d *PrettyLogger) LogFail(l string) {
	d.LogInfoF("❌ %s", l)
}

func (d *PrettyLogger) LogWarnLn(l string) {
	d.LogInfoF(color.New(color.Bold, color.FgHiWhite).Sprintf("❗❗  %s", l))
}

func (d *PrettyLogger) LogJSON(content []byte) {
	d.LogInfoLn(prettyJSON(content))
}

func prettyJSON(content []byte) string {
	result := &bytes.Buffer{}
	if err := json.Indent(result, content, "", "  "); err != nil {
		panic(err)
	}

	return result.String()
}

type SimpleLogger struct {
	logger *logrus.Entry
}

func NewSimpleLogger() *SimpleLogger {
	l := &logrus.Logger{
		Out:   os.Stdout,
		Level: logrus.DebugLevel,
		Formatter: &logrus.TextFormatter{
			DisableColors:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},
	}
	// l.Formatter = &logrus.JSONFormatter{}
	return &SimpleLogger{
		logger: logrus.NewEntry(l),
	}
}

func (d *SimpleLogger) LogProcess(p string, t string, run func() error) error {
	d.logger.WithField("action", "start").WithField("process", p).Infoln(t)
	err := run()
	d.logger.WithField("action", "end").WithField("process", p).Infoln(t)
	return err
}

func (d *SimpleLogger) LogInfoF(format string, a ...interface{}) {
	d.logger.Infof(format, a...)
}

func (d *SimpleLogger) LogInfoLn(a ...interface{}) {
	d.logger.Infoln(a...)
}

func (d *SimpleLogger) LogErrorF(format string, a ...interface{}) {
	d.logger.Errorf(format, a...)
}

func (d *SimpleLogger) LogErrorLn(a ...interface{}) {
	d.logger.Errorln(a...)
}

func (d *SimpleLogger) LogSuccess(l string) {
	d.logger.WithField("status", "SUCCESS").Infoln(l)
}

func (d *SimpleLogger) LogFail(l string) {
	d.logger.WithField("status", "FAIL").Errorln(l)
}

func (d *SimpleLogger) LogWarnLn(l string) {
	d.logger.Warnln(l)
}

func (d *SimpleLogger) LogJSON(content []byte) {
	d.logger.Infoln(string(content))
}

func Process(p string, t string, run func() error) error {
	return defaultLogger.LogProcess(p, t, run)
}

func InfoF(format string, a ...interface{}) {
	defaultLogger.LogInfoF(format, a...)
}

func InfoLn(a ...interface{}) {
	defaultLogger.LogInfoLn(a...)
}

func ErrorF(format string, a ...interface{}) {
	defaultLogger.LogErrorF(format, a...)
}

func ErrorLn(a ...interface{}) {
	defaultLogger.LogErrorLn(a...)
}

func Success(l string) {
	defaultLogger.LogSuccess(l)
}

func Fail(l string) {
	defaultLogger.LogFail(l)
}

func Warning(l string) {
	defaultLogger.LogWarnLn(l)
}

func JSON(content []byte) {
	defaultLogger.LogJSON(content)
}
