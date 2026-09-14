package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/dimension"
	"rankflow/internal/model"
	"rankflow/internal/observability"
	"rankflow/internal/score"
)

type PersistJob struct {
	RankID    int64   `json:"rankId"`
	TypeID    string  `json:"typeId"`
	ItemID    string  `json:"itemId"`
	TraceID   string  `json:"traceId,omitempty"`
	Score     int64   `json:"score"`
	SubScore  int64   `json:"subScore"`
	Final     float64 `json:"final"`
	Revision  int64   `json:"revision"`
	EventTime int64   `json:"eventTime"`
}

type AddScoreInput struct {
	RequestID  string            `json:"requestId"`
	ItemID     string            `json:"itemId"`
	Score      int64             `json:"score"`
	SubScore   int64             `json:"subScore"`
	EventTime  int64             `json:"eventTime"`
	Dimensions map[string]string `json:"dimensions"`
}

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

func (s *Service) prepareBoard(ctx context.Context, rc *ResolvedConfig, rankID int64, typeID string) error {
	return s.rd.MigrateLegacyBoard(ctx, rankID, typeID, &rc.Config)
}

func (s *Service) AddScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil { return nil, err }
	if rc.Config.Status != model.StatusOnline { return nil, ErrNotOnline }
	if in.ItemID == "" { return nil, fmt.Errorf("%w: itemId is required", ErrValidation) }

	anchor := s.anchorTS(rc, in.EventTime)
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, in.Dimensions, anchor)
	if err != nil { return nil, fmt.Errorf("%w: %v", ErrValidation, err) }
	if err := s.requireSubBoardOnline(ctx, rankID, typeID, in.Dimensions); err != nil { return nil, err }
	if err := s.prepareBoard(ctx, rc, rankID, typeID); err != nil { return nil, err }

	if in.RequestID != "" {
		claimed, err := s.rd.ClaimIdempotency(ctx, rankID, in.RequestID, 24*time.Hour)
		if err != nil { return nil, err }
		if !claimed {
			rank, _, err := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
			if err != nil { return nil, err }
			cur, err := s.rd.GetScore(ctx, rankID, typeID, in.ItemID)
			if err != nil { return nil, err }
			final, err := s.rd.GetFinalScore(ctx, rankID, typeID, in.ItemID)
			if err != nil { return nil, err }
			return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: cur, Rank: rank, Final: final}, nil
		}
	}

	encodedMember := score.EncodedMember(&rc.Config, anchor, in.SubScore, in.ItemID)
	subDecimal := score.SubDecimal(&rc.Config, anchor, in.SubScore)
	newScore, revision, final, err := s.rd.AddFinalScore(ctx, rankID, typeID, in.ItemID, encodedMember, in.Score, subDecimal, rc.Config.MaxRankSize, score.IsDesc(&rc.Config))
	if err != nil {
		if in.RequestID != "" { s.rd.ReleaseIdempotency(ctx, rankID, in.RequestID) }
		return nil, err
	}
	if err := s.enqueue(ctx, &PersistJob{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: newScore, SubScore: in.SubScore, Final: final, Revision: revision, EventTime: anchor}); err != nil {
		return nil, err
	}

	rank, _, err := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
	if err != nil { return nil, err }
	s.logger(ctx).Debug("add score succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", in.ItemID), zap.Int64("score", newScore), zap.Int64("revision", revision), zap.Int("rank", rank))
	return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: newScore, Rank: rank, Final: final}, nil
}

func (s *Service) SetScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil { return nil, err }
	if rc.Config.Status != model.StatusOnline { return nil, ErrNotOnline }
	if in.ItemID == "" { return nil, fmt.Errorf("%w: itemId is required", ErrValidation) }
	if in.Score > score.MaxExactBusinessScore || in.Score < -score.MaxExactBusinessScore {
		return nil, fmt.Errorf("%w: score exceeds exact Redis range", ErrValidation)
	}

	anchor := s.anchorTS(rc, in.EventTime)
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, in.Dimensions, anchor)
	if err != nil { return nil, fmt.Errorf("%w: %v", ErrValidation, err) }
	if err := s.requireSubBoardOnline(ctx, rankID, typeID, in.Dimensions); err != nil { return nil, err }
	if err := s.prepareBoard(ctx, rc, rankID, typeID); err != nil { return nil, err }

	final := score.Final(&rc.Config, in.Score, anchor, in.SubScore)
	encodedMember := score.EncodedMember(&rc.Config, anchor, in.SubScore, in.ItemID)
	revision, err := s.rd.SetFinalScore(ctx, rankID, typeID, in.ItemID, encodedMember, in.Score, final, rc.Config.MaxRankSize, score.IsDesc(&rc.Config))
	if err != nil { return nil, err }
	if err := s.enqueue(ctx, &PersistJob{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: in.Score, SubScore: in.SubScore, Final: final, Revision: revision, EventTime: anchor}); err != nil { return nil, err }

	rank, _, err := s.rd.MemberRank(ctx, rankID, typeID, in.ItemID, score.IsDesc(&rc.Config))
	if err != nil { return nil, err }
	return &ScoreResult{RankID: rankID, TypeID: typeID, ItemID: in.ItemID, Score: in.Score, Rank: rank, Final: final}, nil
}

func (s *Service) BatchAddScore(ctx context.Context, rankID int64, items []AddScoreInput) ([]ScoreResult, error) {
	results := make([]ScoreResult, 0, len(items))
	for i := range items {
		r, err := s.AddScore(ctx, rankID, &items[i])
		if err != nil { return results, err }
		results = append(results, *r)
	}
	return results, nil
}

func (s *Service) enqueue(ctx context.Context, job *PersistJob) error {
	if job.TraceID == "" { job.TraceID = observability.TraceID(ctx) }
	payload, err := json.Marshal(job)
	if err != nil { return err }
	if err := s.rd.EnqueuePersist(ctx, string(payload)); err != nil {
		s.logFailure(ctx, "enqueue persist failed", err, zap.Int64("rankId", job.RankID), zap.String("typeId", job.TypeID), zap.String("itemId", job.ItemID), zap.Int64("revision", job.Revision))
		return err
	}
	return nil
}
