package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var L *zap.Logger

func Init() error {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	l, err := cfg.Build()
	if err != nil {
		return err
	}
	L = l
	return nil
}

func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}