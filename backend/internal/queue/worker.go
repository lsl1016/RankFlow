package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
		messages, err := w.rd.ReadPersistMessage(ctx, consumer, pending, 2*time.Second)
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
				ack, retry := w.handleMessage(ctx, msg)
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

func decodePersistJob(msg redis.PersistStreamMessage) (*service.PersistJob, error) {
	if msg.Payload != "" {
		var job service.PersistJob
		if err := json.Unmarshal([]byte(msg.Payload), &job); err != nil {
			return nil, err
		}
		return &job, nil
	}
	if msg.Values["format"] != "v2" {
		return nil, fmt.Errorf("unsupported persist stream format %q", msg.Values["format"])
	}
	parseInt := func(name string) (int64, error) {
		value := msg.Values[name]
		if value == "" {
			return 0, fmt.Errorf("persist stream field %s is required", name)
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse persist stream field %s: %w", name, err)
		}
		return n, nil
	}
	rankID, err := parseInt("rankId")
	if err != nil {
		return nil, err
	}
	score, err := parseInt("score")
	if err != nil {
		return nil, err
	}
	subScore, err := parseInt("subScore")
	if err != nil {
		return nil, err
	}
	revision, err := parseInt("revision")
	if err != nil {
		return nil, err
	}
	eventTime, err := parseInt("eventTime")
	if err != nil {
		return nil, err
	}
	final, err := strconv.ParseFloat(msg.Values["final"], 64)
	if err != nil {
		return nil, fmt.Errorf("parse persist stream field final: %w", err)
	}
	if msg.Values["typeId"] == "" || msg.Values["itemId"] == "" {
		return nil, fmt.Errorf("persist stream typeId and itemId are required")
	}
	return &service.PersistJob{
		RankID:    rankID,
		TypeID:    msg.Values["typeId"],
		ItemID:    msg.Values["itemId"],
		TraceID:   msg.Values["traceId"],
		Score:     score,
		SubScore:  subScore,
		Final:     final,
		Revision:  revision,
		EventTime: eventTime,
	}, nil
}

// handleMessage returns (ack, retry). Malformed messages are acknowledged
// because retrying them can never succeed. Database failures remain pending.
func (w *Worker) handleMessage(ctx context.Context, msg redis.PersistStreamMessage) (bool, bool) {
	job, err := decodePersistJob(msg)
	if err != nil {
		w.log.Warn("decode persist job failed", zap.String("messageId", msg.ID), zap.Error(err))
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
