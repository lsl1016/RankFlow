package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	"rankflow/internal/observability"
	"rankflow/internal/service"
	"rankflow/internal/store/mysql"
	"rankflow/internal/store/redis"
)

type Worker struct {
	rd  *redis.Store
	my  *mysql.Store
	log *zap.Logger
}

func NewWorker(rd *redis.Store, my *mysql.Store, log *zap.Logger) *Worker {
	return &Worker{rd: rd, my: my, log: log}
}

func (w *Worker) Run(ctx context.Context, n int) {
	if n <= 0 {
		n = 1
	}
	if err := w.rd.EnsurePersistGroup(ctx); err != nil {
		w.log.Error("ensure persist stream group failed", zap.Error(err))
		return
	}
	if err := w.rd.MigrateLegacyListQueue(ctx); err != nil {
		w.log.Error("migrate legacy persist queue failed", zap.Error(err))
		return
	}
	for i := 0; i < n; i++ {
		go w.loop(ctx, i)
	}
}

func (w *Worker) loop(ctx context.Context, id int) {
	consumer := fmt.Sprintf("rankflow-%d", id)
	w.log.Info("persist worker started", zap.Int("worker", id), zap.String("consumer", consumer))
	pending := true
	for {
		if ctx.Err() != nil {
			w.log.Info("persist worker stopped", zap.Int("worker", id))
			return
		}
		messages, err := w.rd.ReadPersist(ctx, consumer, pending, 2*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.log.Warn("read persist stream failed", zap.Error(err), zap.String("consumer", consumer))
			time.Sleep(time.Second)
			continue
		}
		if len(messages) == 0 {
			if pending {
				pending = false
			}
			continue
		}
		for _, msg := range messages {
			for {
				ack, retry := w.handle(ctx, msg.Payload)
				if ack {
					if err := w.rd.AckPersist(ctx, msg.ID); err != nil {
						w.log.Error("ack persist message failed", zap.String("messageId", msg.ID), zap.Error(err))
					}
					break
				}
				if !retry || ctx.Err() != nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

// handle returns (ack, retry). Invalid JSON is acknowledged because it can
// never succeed. Database failures remain unacked and are retried in-place.
func (w *Worker) handle(ctx context.Context, payload string) (bool, bool) {
	var job service.PersistJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		w.log.Warn("decode persist job failed", zap.Error(err))
		return true, false
	}
	if job.TraceID != "" {
		ctx = observability.WithTraceID(ctx, job.TraceID)
	}
	log := observability.Logger(ctx, w.log,
		zap.Int64("rankId", job.RankID),
		zap.String("typeId", job.TypeID),
		zap.String("itemId", job.ItemID),
		zap.Int64("revision", job.Revision),
	)
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
		Revision:      job.Revision,
		LastEventTime: eventTime,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := w.my.UpsertMemberScoreIfNewer(ctx, m); err != nil {
		log.Error("upsert member score failed", zap.Error(err))
		return false, true
	}
	log.Debug("persist member score succeeded", zap.Int64("score", job.Score), zap.Int64("subScore", job.SubScore), zap.Float64("final", job.Final))
	return true, false
}
