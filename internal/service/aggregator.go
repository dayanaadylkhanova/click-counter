package service

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type key struct {
	banner int64
	minute int64 // unix minutes since epoch
}

type shard struct {
	mu   sync.Mutex
	data map[key]int64
}

type Aggregator struct {
	log        *zap.Logger
	writer     AggregateWriter
	shards     []shard
	flushEvery time.Duration
	stopCh     chan struct{}
}

func NewAggregator(log *zap.Logger, w AggregateWriter, shardCount int, flushEvery time.Duration) *Aggregator {
	if shardCount <= 0 {
		shardCount = 1
	}
	if flushEvery <= 0 {
		flushEvery = time.Second
	}
	shards := make([]shard, shardCount)
	for i := range shards {
		shards[i] = shard{data: make(map[key]int64, 1024)}
	}
	return &Aggregator{log: log, writer: w, shards: shards, flushEvery: flushEvery, stopCh: make(chan struct{})}
}

func minuteUTC(t time.Time) time.Time { return t.UTC().Truncate(time.Minute) }
func bucket(ts time.Time) int64       { return minuteUTC(ts).Unix() / 60 }

func (a *Aggregator) shardIndex(k key) int {
	x := uint64(k.banner)*1315423911 ^ uint64(k.minute)
	return int(x % uint64(len(a.shards)))
}

func (a *Aggregator) Inc(bannerID int64, now time.Time) {
	k := key{banner: bannerID, minute: bucket(now)}
	sh := &a.shards[a.shardIndex(k)]
	sh.mu.Lock()
	sh.data[k]++
	sh.mu.Unlock()
}

func (a *Aggregator) buildBatchSnapshot() []AggregateRow {
	// Снять снапшот под локами, очистить только после успешной записи
	tmp := make([]map[key]int64, len(a.shards))
	for i := range a.shards {
		sh := &a.shards[i]
		sh.mu.Lock()
		if len(sh.data) > 0 {
			m := make(map[key]int64, len(sh.data))
			for k, v := range sh.data {
				m[k] = v
			}
			tmp[i] = m
		}
		sh.mu.Unlock()
	}
	var batch []AggregateRow
	for i := range tmp {
		if tmp[i] == nil {
			continue
		}
		for k, v := range tmp[i] {
			batch = append(batch, AggregateRow{BannerID: k.banner, TS: time.Unix(k.minute*60, 0).UTC(), Cnt: v})
		}
	}
	return batch
}

func (a *Aggregator) Run(ctx context.Context) {
	t := time.NewTicker(a.flushEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-t.C:
			// Снимем снапшот и попробуем записать
			tmp := make([]map[key]int64, len(a.shards))
			for i := range a.shards {
				sh := &a.shards[i]
				sh.mu.Lock()
				if len(sh.data) > 0 {
					m := make(map[key]int64, len(sh.data))
					for k, v := range sh.data {
						m[k] = v
					}
					tmp[i] = m
				}
				sh.mu.Unlock()
			}
			var batch []AggregateRow
			for i := range tmp {
				if tmp[i] == nil {
					continue
				}
				for k, v := range tmp[i] {
					batch = append(batch, AggregateRow{BannerID: k.banner, TS: time.Unix(k.minute*60, 0).UTC(), Cnt: v})
				}
			}
			if len(batch) == 0 {
				continue
			}
			if err := a.writer.UpsertAggregates(ctx, batch); err != nil {
				a.log.Warn("flush failed", zap.Error(err))
				continue
			}
			// Очистка оригинальных карт только после успеха
			for i := range a.shards {
				sh := &a.shards[i]
				sh.mu.Lock()
				for k := range tmp[i] {
					delete(sh.data, k)
				}
				sh.mu.Unlock()
			}
		}
	}
}

func (a *Aggregator) Stop(ctx context.Context) {
	close(a.stopCh)
	// Финальный flush (best-effort)
	snap := a.buildBatchSnapshot()
	if len(snap) > 0 {
		_ = a.writer.UpsertAggregates(ctx, snap)
	}
}
