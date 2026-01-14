package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() *zap.Logger {
	zapConfig := zap.NewProductionConfig()
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.EncoderConfig.TimeKey = "timestamp"
	zapConfig.DisableCaller = true
	zapConfig.DisableStacktrace = true
	zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		var parsedLevel zap.AtomicLevel
		err := parsedLevel.UnmarshalText([]byte(level))
		if err == nil {
			zapConfig.Level = parsedLevel
		}
	}
	return zap.Must(zapConfig.Build())
}
