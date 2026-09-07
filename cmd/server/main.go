package main

import (
	"log"

	"go.uber.org/zap"

	"github.com/zht475706171/TaiSang-KB/internal/config"
	"github.com/zht475706171/TaiSang-KB/internal/logger"
)

func main() {
	if err := logger.Init(); err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.L.Fatal("config load failed", zap.Error(err))
	}
	logger.L.Info("config loaded",
		zap.String("addr", cfg.ListenAddr),
		zap.String("storage", cfg.StorageDir),
	)
}