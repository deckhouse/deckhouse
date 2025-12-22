package gossip

import (
	"io"

	"go.uber.org/zap"
)

type zapAdapter struct {
	logger *zap.Logger
}


func NewZapAdapter(logger *zap.Logger) io.Writer {
	return &zapAdapter{logger: logger}
}


func (z *zapAdapter) Write(p []byte) (n int, err error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	z.logger.Debug(msg)
	return len(p), nil
}
