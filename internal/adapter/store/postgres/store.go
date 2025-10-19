package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/dayanaadylkhanova/click-counter/internal/entity"
	"github.com/dayanaadylkhanova/click-counter/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Store struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

func New(dsn string, log *zap.Logger) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool, log: log}, nil
}

func (s *Store) Init(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS banner_clicks (
	banner_id BIGINT      NOT NULL,
	ts        TIMESTAMPTZ NOT NULL,
	cnt       BIGINT      NOT NULL,
	PRIMARY KEY (banner_id, ts)
);
CREATE INDEX IF NOT EXISTS idx_banner_clicks_bid_ts ON banner_clicks (banner_id, ts);
`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// UpsertAggregates implements service.AggregateWriter
func (s *Store) UpsertAggregates(ctx context.Context, rows []service.AggregateRow) error {
	if len(rows) == 0 {
		return nil
	}
	sql := "INSERT INTO banner_clicks (banner_id, ts, cnt) VALUES "
	args := make([]any, 0, len(rows)*3)
	for i, r := range rows {
		if i > 0 {
			sql += ","
		}
		o := i*3 + 1
		sql += fmt.Sprintf("($%d,$%d,$%d)", o, o+1, o+2)
		args = append(args, r.BannerID, r.TS, r.Cnt)
	}
	sql += " ON CONFLICT (banner_id, ts) DO UPDATE SET cnt = banner_clicks.cnt + EXCLUDED.cnt"
	_, err := s.pool.Exec(ctx, sql, args...)
	return err
}

// QueryRange implements service.StatsReaderPort
func (s *Store) QueryRange(ctx context.Context, bannerID int64, from, to time.Time) ([]entity.Point, error) {
	const q = `SELECT ts, cnt FROM banner_clicks WHERE banner_id=$1 AND ts >= $2 AND ts < $3 ORDER BY ts`
	rows, err := s.pool.Query(ctx, q, bannerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entity.Point
	for rows.Next() {
		var ts time.Time
		var cnt int64
		if err := rows.Scan(&ts, &cnt); err != nil {
			return nil, err
		}
		out = append(out, entity.Point{TS: ts.UTC(), V: cnt})
	}
	return out, rows.Err()
}

func (s *Store) Close() { s.pool.Close() }
