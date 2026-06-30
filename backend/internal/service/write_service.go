package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/dimension"
	"rankflow/internal/model"
	"rankflow/internal/score"
)

// PersistJob is the payload pushed to the async persist queue and consumed by
// the queue worker to upsert MySQL.
type PersistJob struct {
	RankID    int64   `json:"rankId"`
	TypeID    string  `json:"typeId"`
	ItemID    string  `json:"itemId"`
	Score     int64   `json:"score"`
	SubScore  int64   `json:"subScore"`
	Final     float64 `json:"final"`
	EventTime int64   `json:"eventTime"`
}

// AddScoreInput is a single add-score request.
type AddScoreInput struct {
	RequestID  string            `json:"requestId"`
	ItemID     string            `json:"itemId"`
	Score      int64             `json:"score"`
	SubScore   int64             `json:"subScore"`
	EventTime  int64             `json:"eventTime"`
	Dimensions map[string]string `json:"dimensions"`
}

// ScoreResult is returned after a write.
type ScoreResult struct {
	RankID int64   `json:"rankId"`
	TypeID string  `json:"typeId"`
	ItemID string  `json:"itemId"`
	Score  int64   `json:"score"`
	Rank   int     `json:"rank"`
	Final  float64 `json:"final"`
}

func (s *Service) anchorTS(rc *ResolvedConfig, eventTS int64) int64 {
	if rc.Time.AnchorType == model.AnchorRequestTime || eventTS <= 0 {
		return time.Now().Unix()
	}
	return eventTS
}

// AddScore applies a score delta to a member with idempotency and async
// persistence. Returns the member's new score and rank.
func (s *Service) AddScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		return nil, err
	}
	if rc.Config.Status != model.StatusOnline {
		return nil, ErrNotOnline
	}
	if in.ItemID == "" {
		return nil, fmt.Errorf("%w: itemId is required", ErrValidation)
	}

	anchor := s.anchorTS(rc, in.EventTime)
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, in.Dimensions, anchor)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	// Idempotency: claim the requestId before mutating. On any failure after
	// the claim we release it so the event can be retried.
	claimed := true
	if in.RequestID != "" {
		ttl := 24 * time.Hour
		claimed, err = s.rd.ClaimIdempotency(ctx, rankID, in.RequestID, ttl)
		if err != nil {
			return nil, err
		}
		if !claimed {
			// Duplicate: return current state without re-adding.
			rank, final, _ := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
			cur, _ := s.rd.GetScore(ctx, rankID, typeID, in.ItemID)
			return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: cur, Rank: rank, Final: final}, nil
		}
	}

	subDecimal := score.SubDecimal(&rc.Config, anchor, in.SubScore)
	newScore, err := s.rd.AddFinalScore(ctx, rankID, typeID, in.ItemID, in.Score, subDecimal, rc.Config.MaxRankSize, score.IsDesc(&rc.Config))
	if err != nil {
		if in.RequestID != "" {
			s.rd.ReleaseIdempotency(ctx, rankID, in.RequestID)
		}
		return nil, err
	}

	final := float64(newScore) + subDecimal
	s.enqueue(ctx, &PersistJob{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: newScore, SubScore: in.SubScore, Final: final, EventTime: anchor})

	rank, _, err := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
	if err != nil {
		return nil, err
	}
	return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: newScore, Rank: rank, Final: final}, nil
}

// SetScore overwrites a member's absolute score.
func (s *Service) SetScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		return nil, err
	}
	if rc.Config.Status != model.StatusOnline {
		return nil, ErrNotOnline
	}
	if in.ItemID == "" {
		return nil, fmt.Errorf("%w: itemId is required", ErrValidation)
	}
	anchor := s.anchorTS(rc, in.EventTime)
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, in.Dimensions, anchor)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	final := score.Final(&rc.Config, in.Score, anchor, in.SubScore)
	if err := s.rd.SetFinalScore(ctx, rankID, typeID, in.ItemID, in.Score, final); err != nil {
		return nil, err
	}
	s.enqueue(ctx, &PersistJob{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: in.Score, SubScore: in.SubScore, Final: final, EventTime: anchor})
	rank, _, err := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
	if err != nil {
		return nil, err
	}
	return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: in.Score, Rank: rank, Final: final}, nil
}

// BatchAddScore applies multiple add-score requests, returning per-item results.
func (s *Service) BatchAddScore(ctx context.Context, rankID int64, items []AddScoreInput) ([]ScoreResult, error) {
	results := make([]ScoreResult, 0, len(items))
	for i := range items {
		r, err := s.AddScore(ctx, rankID, &items[i])
		if err != nil {
			return results, err
		}
		results = append(results, *r)
	}
	return results, nil
}

func (s *Service) enqueue(ctx context.Context, job *PersistJob) {
	payload, err := json.Marshal(job)
	if err != nil {
		return
	}
	if err := s.rd.EnqueuePersist(ctx, string(payload)); err != nil {
		s.log.Warn("enqueue persist failed", zap.Int64("rankId", job.RankID), zap.Error(err))
	}
}
