package log

type Logger interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}
