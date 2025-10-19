package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

// helper: wait with timeout for a signal
func waitCh[T any](t *testing.T, ch <-chan T, d time.Duration) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		t.Fatalf("timeout waiting for channel")
		return *new(T)
	}
}

func TestAggregator_IncAndFlush_SingleMinute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockW := NewMockAggregateWriter(ctrl)
	log := zap.NewNop()

	flushEvery := 10 * time.Millisecond
	agg := NewAggregator(log, mockW, 8, flushEvery)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gotRows := make(chan []AggregateRow, 1)

	mockW.EXPECT().
		UpsertAggregates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []AggregateRow) error {
			cp := append([]AggregateRow(nil), rows...)
			gotRows <- cp
			return nil
		}).
		Times(1)

	go agg.Run(ctx)

	// 2 clicks in the same minute bucket
	now := time.Date(2025, 10, 19, 0, 29, 42, 0, time.UTC)
	agg.Inc(1, now)
	agg.Inc(1, now.Add(10*time.Second))

	rows := waitCh(t, gotRows, 300*time.Millisecond)
	if len(rows) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(rows))
	}
	r := rows[0]
	if r.BannerID != 1 {
		t.Fatalf("expected BannerID=1, got %d", r.BannerID)
	}
	if !r.TS.Equal(now.Truncate(time.Minute)) {
		t.Fatalf("expected TS=%s, got %s", now.Truncate(time.Minute), r.TS)
	}
	if r.Cnt != 2 {
		t.Fatalf("expected Cnt=2, got %d", r.Cnt)
	}

	// After successful flush, internal state should be cleared; no second write without new Incs.
	time.Sleep(2 * flushEvery)
	cancel()
	agg.Stop(context.Background())
}

func TestAggregator_FlushFailure_DoesNotClear(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockW := NewMockAggregateWriter(ctrl)
	log := zap.NewNop()

	flushEvery := 10 * time.Millisecond
	agg := NewAggregator(log, mockW, 4, flushEvery)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type snap struct{ rows []AggregateRow }
	snaps := make(chan snap, 2)

	// 1st call fails, 2nd succeeds. We capture both snapshots;
	// they must be identical (state is not cleared on failure).
	gomock.InOrder(
		mockW.EXPECT().
			UpsertAggregates(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, rows []AggregateRow) error {
				cp := append([]AggregateRow(nil), rows...)
				snaps <- snap{rows: cp}
				return assertErr // any non-nil error
			}),
		mockW.EXPECT().
			UpsertAggregates(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, rows []AggregateRow) error {
				cp := append([]AggregateRow(nil), rows...)
				snaps <- snap{rows: cp}
				return nil
			}),
	)

	go agg.Run(ctx)

	now := time.Date(2025, 10, 19, 0, 29, 0, 0, time.UTC)
	// total 5 clicks for minute 00:29
	for i := 0; i < 5; i++ {
		agg.Inc(42, now.Add(time.Duration(i)*time.Second))
	}

	first := waitCh(t, snaps, 500*time.Millisecond)
	second := waitCh(t, snaps, 500*time.Millisecond)

	if len(first.rows) != 1 || len(second.rows) != 1 {
		t.Fatalf("expected single row in each flush, got %d and %d", len(first.rows), len(second.rows))
	}
	fr, sr := first.rows[0], second.rows[0]
	if !fr.TS.Equal(now) || !sr.TS.Equal(now) || fr.BannerID != 42 || sr.BannerID != 42 {
		t.Fatalf("unexpected rows: %#v vs %#v", fr, sr)
	}
	if fr.Cnt != 5 || sr.Cnt != 5 {
		t.Fatalf("expected same count 5 after failure, got %d then %d", fr.Cnt, sr.Cnt)
	}

	cancel()
	agg.Stop(context.Background())
}

// assertErr is a sentinel error for mock failure branch.
type assertErrType struct{}

func (assertErrType) Error() string { return "assert: expected failure" }

var assertErr = assertErrType{}

func TestAggregator_Stop_FlushesBestEffort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockW := NewMockAggregateWriter(ctrl)
	log := zap.NewNop()

	agg := NewAggregator(log, mockW, 2, time.Hour) // long ticker, rely on Stop()
	now := time.Date(2025, 10, 19, 0, 29, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		agg.Inc(7, now.Add(time.Duration(i)*5*time.Second))
	}

	done := make(chan struct{}, 1)
	mockW.EXPECT().
		UpsertAggregates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []AggregateRow) error {
			if len(rows) != 1 {
				t.Fatalf("expected 1 row on Stop flush, got %d", len(rows))
			}
			if rows[0].Cnt != 3 {
				t.Fatalf("expected Cnt=3 on Stop flush, got %d", rows[0].Cnt)
			}
			done <- struct{}{}
			return nil
		}).
		Times(1)

	// We don't run Run(); Stop must flush snapshot anyway.
	agg.Stop(context.Background())
	waitCh(t, done, 300*time.Millisecond)
}

func TestAggregator_ConcurrentInc_CountsAreAccurate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockW := NewMockAggregateWriter(ctrl)
	log := zap.NewNop()

	agg := NewAggregator(log, mockW, 64, time.Hour) // no periodic flush
	now := time.Date(2025, 10, 19, 0, 29, 0, 0, time.UTC)

	workers, per := 16, 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				agg.Inc(1001, now)
			}
		}()
	}
	wg.Wait()

	done := make(chan struct{}, 1)
	mockW.EXPECT().
		UpsertAggregates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rows []AggregateRow) error {
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			want := int64(workers * per)
			if rows[0].Cnt != want {
				t.Fatalf("expected Cnt=%d, got %d", want, rows[0].Cnt)
			}
			done <- struct{}{}
			return nil
		}).
		Times(1)

	agg.Stop(context.Background())
	waitCh(t, done, 300*time.Millisecond)
}
