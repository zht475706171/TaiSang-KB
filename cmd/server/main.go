package main

import (
	"log"

	"go.uber.org/zap"

	"github.com/zht475706171/TaiSang-KB/internal/config"
	"github.com/zht475706171/TaiSang-KB/internal/logger"
	"github.com/zht475706171/TaiSang-KB/internal/router"
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

	// TODO Task 6: wire database after DB migrate task done.
	// Task 6 (internal/database Connect/Migrate) was skipped in this run, so
	// main.go currently boots without a DB connection. Health endpoint does
	// not need a DB. Re-enable the block below once internal/database lands:
	//
	// db, err := database.Connect(cfg.DBDSN)
	// if err != nil {
	// 	logger.L.Fatal("db connect failed", zap.Error(err))
	// }
	// if err := database.Migrate(db); err != nil {
	// 	logger.L.Fatal("db migrate failed", zap.Error(err))
	// }
	// logger.L.Info("db migrated")
	// r := router.New(db)

	r := router.New(nil) // nil db until Task 6 wires database
	logger.L.Info("listening", zap.String("addr", cfg.ListenAddr))
	if err := r.Run(cfg.ListenAddr); err != nil {
		logger.L.Fatal("server stopped", zap.Error(err))
	}
}