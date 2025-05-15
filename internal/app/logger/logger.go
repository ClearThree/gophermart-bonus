package logger

import (
	"github.com/ClearThree/gophermart-bonus/internal/app/config"
	"go.uber.org/zap"
	"log"
)

var Log *zap.SugaredLogger

func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	logger, err := cfg.Build()
	if err != nil {
		return err
	}
	Log = logger.Sugar()
	return nil
}

func init() {
	err := Initialize(config.Settings.LogLevel)
	if err != nil {
		log.Fatal("error initializing logger")
	}
}
