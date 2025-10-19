package service

import (
	"context"
	"time"

	"github.com/dayanaadylkhanova/click-counter/internal/entity"
)

type AggregatorPort interface {
	Inc(bannerID int64, now time.Time)
	Run(ctx context.Context)
	Stop(ctx context.Context)
}

type StatsReaderPort interface {
	QueryRange(ctx context.Context, bannerID int64, from, to time.Time) ([]entity.Point, error)
}

// AggregateWriter — порт для записи агрегированных значений в БД.
type AggregateWriter interface {
	UpsertAggregates(ctx context.Context, rows []AggregateRow) error
}

// AggregateRow — одна строка агрегата (поминутная).
type AggregateRow struct {
	BannerID int64
	TS       time.Time // начало минуты (UTC)
	Cnt      int64
}
