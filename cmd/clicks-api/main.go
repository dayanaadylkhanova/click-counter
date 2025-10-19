package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/dayanaadylkhanova/click-counter/internal/app"
	"github.com/dayanaadylkhanova/click-counter/pkg/config"
	"github.com/dayanaadylkhanova/click-counter/pkg/logger"
	"go.uber.org/zap"
)

var (
	AppName      = "clicks-api"
	AppBuildTime = "dev"
	AppCommit    = "dev"
	AppRelease   = "dev"
)

func main() {
	// 1) Конфиг
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("can't parse app config: %v", err)
	}
	if cfg.MaxCPU > 0 {
		runtime.GOMAXPROCS(cfg.MaxCPU)
	}

	// 2) Информация о приложении
	info := &app.AppInfo{
		Name:      AppName,
		BuildTime: AppBuildTime,
		Commit:    AppCommit,
		Release:   AppRelease,
	}

	// 3) Логгер (zap)
	zl := logger.NewJSON(cfg.LogLevel)
	defer func() {
		if r := recover(); r != nil {
			zl.Error("panic error", zap.Error(fmt.Errorf("%v", r)))
		}
		_ = zl.Sync()
	}()
	zap.ReplaceGlobals(zl)
	zl.Info(fmt.Sprintf("Application `%s` %s started.", AppName, AppRelease))

	// 4) Контекст и обработка сигналов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	// 5) Запуск приложения
	application, err := app.New(*cfg, info, zl)
	if err != nil {
		zl.Fatal("can't build app", zap.Error(err))
	}
	if err := application.Run(ctx); err != nil {
		switch {
		case errors.Is(err, app.ErrAppStartup):
			zl.Error("can't run application", zap.Error(err))
		case errors.Is(err, app.ErrAppShutdownWithError):
			zl.Error("application is shutdown with error", zap.Error(err))
		case errors.Is(err, app.ErrAppShutdownNormal):
			fallthrough
		default:
			zl.Warn("application is shutdown")
		}
	}

	// 6) Чуть-чуть времени на Sync
	time.Sleep(100 * time.Millisecond)
}
