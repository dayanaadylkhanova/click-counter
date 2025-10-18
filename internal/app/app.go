package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/example/click-counter/internal/adapter/store/postgres"
	http_server "github.com/example/click-counter/internal/adapter/transport/http"
	"github.com/example/click-counter/internal/service"
	"github.com/example/click-counter/pkg/config"
	"go.uber.org/zap"
)

type AppInfo struct {
	Name      string
	BuildTime string
	Commit    string
	Release   string
}

type App struct {
	cfg  config.Config
	info *AppInfo
	log  *zap.Logger

	store      *postgres.Store
	aggregator *service.Aggregator
	server     *http_server.Server
}

func New(cfg config.Config, info *AppInfo, log *zap.Logger) (*App, error) {
	// 1) Store (Postgres)
	st, err := postgres.New(cfg.DatabaseURL, log)
	if err != nil {
		return nil, err
	}
	if err := st.Init(context.Background()); err != nil {
		return nil, err
	}

	// 2) Aggregator
	agg := service.NewAggregator(log, st, cfg.Shards, cfg.FlushEvery)

	// 3) HTTP server (ports: AggregatorPort + StatsReaderPort)
	srv := http_server.NewServer(log, cfg.ListenAddr, agg, st, cfg.ReadMaxRangeDays)

	return &App{
		cfg:        cfg,
		info:       info,
		log:        log,
		store:      st,
		aggregator: agg,
		server:     srv,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	// Start background aggregator flush
	bgCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go a.aggregator.Run(bgCtx)

	// Start HTTP
	httpErrCh := make(chan error, 1)
	go func() { httpErrCh <- a.server.Start() }()

	var runErr error
	select {
	case <-ctx.Done():
		// graceful
		runErr = ErrAppShutdownNormal
	case err := <-httpErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErr = ErrAppStartup
		} else {
			runErr = ErrAppShutdownNormal
		}
	}

	// Graceful shutdown
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), a.cfg.ShutdownWait)
	defer cancelShutdown()
	_ = a.server.Shutdown(shutdownCtx)
	a.aggregator.Stop(shutdownCtx)
	a.store.Close()

	return runErr
}
