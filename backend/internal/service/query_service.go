package service

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"rankflow/internal/dimension"
	"rankflow/internal/score"
	mysqlstore "rankflow/internal/store/mysql"
	"rankflow/internal/store/redis"
)

type TopResult struct {
	RankID int64             `json:"rankId"`
	TypeID string            `json:"typeId"`
	Total  int64             `json:"total"`
	Items  []redis.RankEntry `json:"items"`
}

// computeTypeID resolves configuration and computes a subboard identifier. It
// has no subboard persistence side effects and is shared by read and admin paths.
func (s *Service) computeTypeID(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*ResolvedConfig, string, error) {
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

// readBoard resolves an existing subboard without creating or updating its
// MySQL metadata. A missing subboard is represented by exists=false and callers
// return an empty rank result instead of materializing data during a GET.
func (s *Service) readBoard(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*ResolvedConfig, string, bool, error) {
	rc, typeID, err := s.computeTypeID(ctx, rankID, dims, ts)
	if err != nil {
		return nil, "", false, err
	}
	if _, err := s.my.GetSubBoard(ctx, rankID, typeID); err != nil {
		if errors.Is(err, mysqlstore.ErrNotFound) {
			return rc, typeID, false, nil
		}
		return nil, "", false, err
	}
	if err := s.prepareBoard(ctx, rc, rankID, typeID); err != nil {
		return nil, "", false, err
	}
	return rc, typeID, true, nil
}

func (s *Service) QueryTop(ctx context.Context, rankID int64, dims map[string]string, ts int64, offset, limit int) (*TopResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rc, typeID, exists, err := s.readBoard(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &TopResult{RankID: rankID, TypeID: typeID, Total: 0, Items: []redis.RankEntry{}}, nil
	}
	items, err := s.rd.Top(ctx, rankID, typeID, offset, limit, score.IsDesc(&rc.Config))
	if err != nil {
		return nil, err
	}
	total, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		return nil, err
	}
	s.logger(ctx).Debug("query top succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int64("total", total))
	return &TopResult{RankID: rankID, TypeID: typeID, Total: total, Items: items}, nil
}

type MemberRankResult struct {
	RankID int64   `json:"rankId"`
	TypeID string  `json:"typeId"`
	ItemID string  `json:"itemId"`
	Score  float64 `json:"score"`
	Rank   int     `json:"rank"`
}

func (s *Service) QueryMemberRank(ctx context.Context, rankID int64, itemID string, dims map[string]string, ts int64) (*MemberRankResult, error) {
	rc, typeID, exists, err := s.readBoard(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &MemberRankResult{RankID: rankID, TypeID: typeID, ItemID: itemID, Score: 0, Rank: -1}, nil
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
	if before > 100 {
		before = 100
	}
	if after > 100 {
		after = 100
	}
	rc, typeID, exists, err := s.readBoard(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &TopResult{RankID: rankID, TypeID: typeID, Total: 0, Items: []redis.RankEntry{}}, nil
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

type StatsResult struct {
	RankID      int64  `json:"rankId"`
	TypeID      string `json:"typeId"`
	MemberCount int64  `json:"memberCount"`
}

func (s *Service) Stats(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*StatsResult, error) {
	_, typeID, exists, err := s.readBoard(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &StatsResult{RankID: rankID, TypeID: typeID, MemberCount: 0}, nil
	}
	count, err := s.rd.Card(ctx, rankID, typeID)
	if err != nil {
		return nil, err
	}
	return &StatsResult{RankID: rankID, TypeID: typeID, MemberCount: count}, nil
}
