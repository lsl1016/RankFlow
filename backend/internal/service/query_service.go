package service

import (
	"context"
	"fmt"

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
		return nil, "", err
	}
	anchor := ts
	if anchor <= 0 {
		anchor = s.anchorTS(rc, 0)
	}
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, dims, anchor)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrValidation, err)
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
		return nil, err
	}
	total, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		return nil, err
	}
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
		return nil, err
	}
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
		return nil, err
	}
	total, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		return nil, err
	}
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
		return nil, err
	}
	return &StatsResult{RankID: rankID, TypeID: typeID, MemberCount: count}, nil
}
