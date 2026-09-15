package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	"rankflow/internal/store/mysql"
)

const subBoardStatusCacheTTL = 5 * time.Minute

type SubBoard struct {
	RankID      int64             `json:"rankId"`
	TypeID      string            `json:"typeId"`
	Dimensions  map[string]string `json:"dimensions"`
	Status      int               `json:"status"`
	MemberCount int64             `json:"memberCount"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

func (s *Service) ResolveSubBoard(ctx context.Context, rankID int64, dims map[string]string, ts int64) (*SubBoard, error) {
	rc, typeID, err := s.computeTypeID(ctx, rankID, dims, ts)
	if err != nil {
		return nil, err
	}
	if err := s.ensureSubBoard(ctx, rankID, typeID, dims); err != nil {
		return nil, err
	}
	if err := s.prepareBoard(ctx, rc, rankID, typeID); err != nil {
		return nil, err
	}
	sb, err := s.GetSubBoard(ctx, rankID, typeID)
	if err != nil {
		return nil, err
	}
	s.cacheSubBoardStatus(ctx, rankID, typeID, sb.Status)
	return sb, nil
}

func (s *Service) ListSubBoards(ctx context.Context, rankID int64) ([]SubBoard, error) {
	rows, err := s.my.ListSubBoards(ctx, rankID)
	if err != nil {
		s.logFailure(ctx, "list sub boards failed", err, zap.Int64("rankId", rankID))
		return nil, err
	}
	out := make([]SubBoard, 0, len(rows))
	for _, row := range rows {
		sb, err := s.subBoardFromModel(ctx, row)
		if err != nil {
			s.logger(ctx).Warn("decode sub board failed", zap.Int64("rankId", row.RankID), zap.String("typeId", row.TypeID), zap.Error(err))
			continue
		}
		out = append(out, *sb)
	}
	return out, nil
}

func (s *Service) GetSubBoard(ctx context.Context, rankID int64, typeID string) (*SubBoard, error) {
	row, err := s.my.GetSubBoard(ctx, rankID, typeID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return nil, ErrNotFound
		}
		s.logFailure(ctx, "get sub board failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID))
		return nil, err
	}
	return s.subBoardFromModel(ctx, *row)
}

func (s *Service) SetSubBoardStatus(ctx context.Context, rankID int64, typeID string, status int) error {
	if status != model.StatusOnline && status != model.StatusOffline {
		err := fmt.Errorf("%w: invalid sub board status", ErrValidation)
		s.logFailure(ctx, "set sub board status validation failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("status", status))
		return err
	}
	if _, err := s.my.GetSubBoard(ctx, rankID, typeID); err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Delete the old cache entry before changing MySQL. If Redis is currently
	// unavailable, fail before mutating the source of truth rather than leaving
	// an old ONLINE cache entry that could temporarily accept writes.
	if err := s.rd.DelSubBoardStatusCache(ctx, rankID, typeID); err != nil {
		s.logFailure(ctx, "invalidate sub board status cache failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID))
		return err
	}
	if err := s.my.UpdateSubBoardStatus(ctx, rankID, typeID, status); err != nil {
		s.logFailure(ctx, "update sub board status failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("status", status))
		return err
	}
	// The cache was already removed, so a failed refill is safe: the next write
	// falls back to MySQL instead of observing stale status.
	if err := s.rd.SetSubBoardStatusCache(ctx, rankID, typeID, status, subBoardStatusCacheTTL); err != nil {
		s.logger(ctx).Warn("cache sub board status failed", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("status", status), zap.Error(err))
	}
	s.logger(ctx).Info("set sub board status succeeded", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("status", status))
	return nil
}

func (s *Service) ensureSubBoard(ctx context.Context, rankID int64, typeID string, dims map[string]string) error {
	if dims == nil {
		dims = map[string]string{}
	}
	payload, err := json.Marshal(dims)
	if err != nil {
		s.logFailure(ctx, "encode sub board dimensions failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Any("dimensions", dims))
		return err
	}
	now := time.Now()
	sb := &model.RankSubBoard{
		RankID:     rankID,
		TypeID:     typeID,
		Dimensions: string(payload),
		Status:     model.StatusOnline,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.my.UpsertSubBoard(ctx, sb); err != nil {
		s.logFailure(ctx, "upsert sub board failed", err, zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Any("dimensions", dims))
		return err
	}
	return nil
}

func (s *Service) requireSubBoardOnline(ctx context.Context, rankID int64, typeID string, dims map[string]string) error {
	status, cached, err := s.rd.GetSubBoardStatusCache(ctx, rankID, typeID)
	if err == nil && cached {
		if status != model.StatusOnline {
			return ErrNotOnline
		}
		return nil
	}
	if err != nil {
		s.logger(ctx).Warn("read sub board status cache failed", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Error(err))
	}

	// Cache miss: read MySQL once. Only a genuinely missing subboard is
	// materialized; existing subboards are no longer Upserted on every score
	// write, so rank_sub_board.updated_at stays metadata rather than write QPS.
	sb, err := s.my.GetSubBoard(ctx, rankID, typeID)
	if errors.Is(err, mysql.ErrNotFound) {
		if err := s.ensureSubBoard(ctx, rankID, typeID, dims); err != nil {
			return err
		}
		// Re-read after the idempotent upsert so a concurrent admin status change
		// is respected instead of assuming the row is still ONLINE.
		sb, err = s.my.GetSubBoard(ctx, rankID, typeID)
	}
	if err != nil {
		return err
	}

	s.cacheSubBoardStatus(ctx, rankID, typeID, sb.Status)
	if sb.Status != model.StatusOnline {
		return ErrNotOnline
	}
	return nil
}

func (s *Service) cacheSubBoardStatus(ctx context.Context, rankID int64, typeID string, status int) {
	if err := s.rd.SetSubBoardStatusCache(ctx, rankID, typeID, status, subBoardStatusCacheTTL); err != nil {
		s.logger(ctx).Warn("cache sub board status failed", zap.Int64("rankId", rankID), zap.String("typeId", typeID), zap.Int("status", status), zap.Error(err))
	}
}

func (s *Service) subBoardFromModel(ctx context.Context, row model.RankSubBoard) (*SubBoard, error) {
	dims := map[string]string{}
	if row.Dimensions != "" {
		if err := json.Unmarshal([]byte(row.Dimensions), &dims); err != nil {
			return nil, err
		}
	}
	if dims == nil {
		dims = map[string]string{}
	}
	count, err := s.rd.Card(ctx, row.RankID, row.TypeID)
	if err != nil {
		s.logger(ctx).Warn("count sub board members failed", zap.Int64("rankId", row.RankID), zap.String("typeId", row.TypeID), zap.Error(err))
	}
	return &SubBoard{
		RankID:      row.RankID,
		TypeID:      row.TypeID,
		Dimensions:  dims,
		Status:      row.Status,
		MemberCount: count,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}
