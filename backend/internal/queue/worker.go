package queue

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	"rankflow/internal/service"
	"rankflow/internal/store/mysql"
	"rankflow/internal/store/redis"
)

// Worker drains the Redis persist queue and upserts member scores into MySQL.
// This keeps Redis on the hot write path while MySQL is updated asynchronously.
type Worker struct {
	rd  *redis.Store
	my  *mysql.Store
	log *zap.Logger
}

func NewWorker(rd *redis.Store, my *mysql.Store, log *zap.Logger) *Worker {
	return &Worker{rd: rd, my: my, log: log}
}

// Run launches n worker goroutines that block until ctx is cancelled.
func (w *Worker) Run(ctx context.Context, n int) {
	if n <= 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		go w.loop(ctx, i)
	}
}

func (w *Worker) loop(ctx context.Context, id int) {
	w.log.Info("persist worker started", zap.Int("worker", id))
	for {
		select {
		case <-ctx.Done():
			w.log.Info("persist worker stopped", zap.Int("worker", id))
			return
		default:
		}

		payload, err := w.rd.DequeuePersist(ctx, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Warn("dequeue persist failed", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if payload == "" {
			continue
		}
		w.handle(ctx, payload)
	}
}

func (w *Worker) handle(ctx context.Context, payload string) {
	var job service.PersistJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		w.log.Warn("decode persist job failed", zap.Error(err))
		return
	}
	var eventTime *time.Time
	if job.EventTime > 0 {
		t := time.Unix(job.EventTime, 0)
		eventTime = &t
	}
	now := time.Now()
	m := &model.RankMemberScore{
		RankID:        job.RankID,
		TypeID:        job.TypeID,
		ItemID:        job.ItemID,
		Score:         job.Score,
		SubScore:      job.SubScore,
		FinalScore:    job.Final,
		LastEventTime: eventTime,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := w.my.UpsertMemberScore(ctx, m); err != nil {
		w.log.Warn("upsert member score failed",
			zap.Int64("rankId", job.RankID), zap.String("itemId", job.ItemID), zap.Error(err))
		// Re-enqueue once for at-least-once delivery.
		_ = w.rd.EnqueuePersist(ctx, payload)
		time.Sleep(200 * time.Millisecond)
	}
}
