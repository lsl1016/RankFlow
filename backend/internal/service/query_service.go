package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"rankflow/internal/dimension"
	"rankflow/internal/score"
	"rankflow/internal/store/redis"
)

// TopResult is a leaderboard page.
type TopResult struct {
	RankID int64             `json:"rankId"`
	TypeID string            `json:"typeId"`
	Total  int64             `json:"total"`
	Items  []redis.RankEntry `json:"items"`
}

func (s *Service) typeIDFor(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*ResolvedConfig, string, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		s.logFailure(ctx, "resolve rank type failed", err, zap.Int64("rankId", rankID), zap.Any("dimensions", dims), zap.Int64("timestamp", ts))
		return nil, "", err
	}
	anchor := ts
	if anchor <= 0 {
		anchor = s.anchorTS(rc, 0)
	}
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, dims, anchor)
	if err != nil {
		wrapped := fmt.Errorf("%w: %v", ErrValidation, err)
		s.logFailure(ctx, "compute rank type failed", wrapped, zap.Int64("rankId", rankID), zap.Any("dimensions", dims), zap.Int64("anchor", anchor))
		return nil, "", wrapped
	}
	return rc, typeID, nil
}

// QueryTop returns a page of the leaderboard.
func (s *Service) QueryTop(ctx context.Context, rankID int64, dims map[string]string, ts int64, offset, limit int) (*TopResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rc, typeID, err := s.typeIDFor(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	items, err := s.rd.Top(ctx, rankID, typeID, offset, limit, score.IsDesc(&rc.Config))
	if err != nil {
		s.logFailure(ctx, "query top failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("offset", offset), zap.Int("limit", limit), zap.Any("dimensions", dims))
		return nil, err
	}
	total, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		s.logFailure(ctx, "query top total failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Any("dimensions", dims))
		return nil, err
	}
	s.logger(ctx).Info("query top succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("offset", offset), zap.Int("limit", limit), zap.Int64("total", total), zap.Int("itemCount", len(items)))
	return &TopResult{RankID: rankID, TypeID: typeID, Total: total, Items: items}, nil
}

// MemberRankResult is a single member's standing.
type MemberRankResult struct {
	RankID int64   `json:"rankId"`
	TypeID string  `json:"typeId"`
	ItemID string  `json:"itemId"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

func (s *Service) QueryMemberRank(ctx context.Context, rankID int64, itemID string, dims map[string]string, ts int64) (*MemberRankResult, error) {
	rc, typeID, err := s.typeIDFor(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	rank, sc, err := s.rd.MemberRank(ctx, rankID, typeID, itemID, score.IsDesc(&rc.Config))
	if err != nil {
		s.logFailure(ctx, "query member rank failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", itemID), zap.Any("dimensions", dims))
		return nil, err
	}
	s.logger(ctx).Info("query member rank succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", itemID), zap.Int("rank", rank), zap.Float64("score", sc))
	return &MemberRankResult{RankID: rankID, TypeID: typeID, ItemID: itemID, Score: sc, Rank: rank}, nil
}

func (s *Service) QueryAround(ctx context.Context, rankID int64, itemID string, dims map[string]string, ts int64, before, after int) (*TopResult, error) {
	if before < 0 {
		before = 5
	}
	if after < 0 {
		after = 5
	}
	rc, typeID, err := s.typeIDFor(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	items, err := s.rd.Around(ctx, rankID, typeID, itemID, before, after, score.IsDesc(&rc.Config))
	if err != nil {
		s.logFailure(ctx, "query around failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", itemID), zap.Int("before", before), zap.Int("after", after), zap.Any("dimensions", dims))
		return nil, err
	}
	total, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		s.logFailure(ctx, "query around total failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", itemID), zap.Any("dimensions", dims))
		return nil, err
	}
	s.logger(ctx).Info("query around succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.String("itemId", itemID), zap.Int("before", before), zap.Int("after", after), zap.Int64("total", total), zap.Int("itemCount", len(items)))
	return &TopResult{RankID: rankID, TypeID: typeID, Total: total, Items: items}, nil
}

// StatsResult powers the rank detail page overview.
type StatsResult struct {
	RankID      int64  `json:"rankId"`
	TypeID      string `json:"typeId"`
	MemberCount int64  `json:"memberCount"`
}

func (s *Service) Stats(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*StatsResult, error) {
	_, typeID, err := s.typeIDFor(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	count, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		s.logFailure(ctx, "query stats failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Any("dimensions", dims))
		return nil, err
	}
	s.logger(ctx).Info("query stats succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int64("memberCount", count))
	return &StatsResult{RankID: rankID, TypeID: typeID, MemberCount: count}, nil
}
