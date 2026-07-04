package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	"rankflow/internal/store/mysql"
)

// DimensionInput is a dimension definition supplied when creating/editing a rank.
type DimensionInput struct {
	DimensionName  string `json:"dimensionName"`
	DimensionField string `json:"dimensionField"`
	Required       bool   `json:"required"`
}

// CreateRankInput captures everything needed to define a leaderboard.
type CreateRankInput struct {
	RankName        string           `json:"rankName"`
	BizCode         string           `json:"bizCode"`
	TargetType      string           `json:"targetType"`
	SortType        string           `json:"sortType"`
	SameScorePolicy string           `json:"sameScorePolicy"`
	MaxRankSize     int              `json:"maxRankSize"`
	CacheTTLSeconds int              `json:"cacheTtlSeconds"`
	TimeType        string           `json:"timeType"`
	Timezone        string           `json:"timezone"`
	AnchorType      string           `json:"anchorType"`
	StartTime       *time.Time       `json:"startTime"`
	EndTime         *time.Time       `json:"endTime"`
	Dimensions      []DimensionInput `json:"dimensions"`
	Online          bool             `json:"online"`
}

func (in *CreateRankInput) normalize() error {
	if in.RankName == "" || in.BizCode == "" || in.TargetType == "" {
		return fmt.Errorf("%w: rankName, bizCode, targetType are required", ErrValidation)
	}
	if in.SortType == "" {
		in.SortType = model.SortScoreDesc
	}
	if in.SortType != model.SortScoreDesc && in.SortType != model.SortScoreAsc {
		return fmt.Errorf("%w: invalid sortType", ErrValidation)
	}
	if in.SameScorePolicy == "" {
		in.SameScorePolicy = model.SameScoreEarlyFirst
	}
	if in.TimeType == "" {
		in.TimeType = model.TimeNone
	}
	if in.Timezone == "" {
		in.Timezone = "Asia/Shanghai"
	}
	if in.AnchorType == "" {
		in.AnchorType = model.AnchorEventTime
	}
	if in.MaxRankSize <= 0 {
		in.MaxRankSize = 10000
	}
	if in.CacheTTLSeconds <= 0 {
		in.CacheTTLSeconds = 3600
	}
	return nil
}

func toDimensionModels(dims []DimensionInput) []model.RankDimensionConfig {
	out := make([]model.RankDimensionConfig, 0, len(dims))
	now := time.Now()
	for i, d := range dims {
		req := 0
		if d.Required {
			req = 1
		}
		out = append(out, model.RankDimensionConfig{
			DimensionName:  d.DimensionName,
			DimensionField: d.DimensionField,
			DimensionOrder: i,
			Required:       req,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	return out
}

// CreateRank allocates a new rank_id and persists the full configuration.
func (s *Service) CreateRank(ctx context.Context, in *CreateRankInput) (int64, error) {
	if err := in.normalize(); err != nil {
		s.logFailure(ctx, "create rank validation failed", err, zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode))
		return 0, err
	}
	maxID, err := s.my.MaxRankID(ctx)
	if err != nil {
		s.logFailure(ctx, "allocate rank id failed", err, zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode))
		return 0, err
	}
	rankID := maxID + 1
	if rankID < 10001 {
		rankID = 10001
	}
	now := time.Now()
	status := model.StatusDraft
	if in.Online {
		status = model.StatusOnline
	}
	cfg := &model.RankConfig{
		RankID:             rankID,
		RankName:           in.RankName,
		BizCode:            in.BizCode,
		TargetType:         in.TargetType,
		Status:             status,
		SortType:           in.SortType,
		SameScorePolicy:    in.SameScorePolicy,
		ScoreIntegerDigits: 12,
		MaxRankSize:        in.MaxRankSize,
		CacheTTLSeconds:    in.CacheTTLSeconds,
		StartTime:          in.StartTime,
		EndTime:            in.EndTime,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	tc := &model.RankTimeConfig{
		TimeType:   in.TimeType,
		Timezone:   in.Timezone,
		AnchorType: in.AnchorType,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	dims := toDimensionModels(in.Dimensions)
	if err := s.my.CreateRank(ctx, cfg, dims, tc); err != nil {
		s.logFailure(ctx, "create rank failed", err, zap.Int64("rankId", rankID), zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode), zap.Int("dimensionCount", len(dims)))
		return 0, err
	}
	s.logger(ctx).Info("create rank succeeded", zap.Int64("rankId", rankID), zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode), zap.Int("status", status), zap.Int("dimensionCount", len(dims)))
	return rankID, nil
}

// UpdateRank rewrites the configuration of an existing rank and invalidates the
// config cache.
func (s *Service) UpdateRank(ctx context.Context, rankID int64, in *CreateRankInput) error {
	if err := in.normalize(); err != nil {
		s.logFailure(ctx, "update rank validation failed", err, zap.Int64("rankId", rankID), zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode))
		return err
	}
	existing, err := s.my.GetRank(ctx, rankID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			s.logFailure(ctx, "update rank target not found", ErrNotFound, zap.Int64("rankId", rankID))
			return ErrNotFound
		}
		s.logFailure(ctx, "load rank before update failed", err, zap.Int64("rankId", rankID))
		return err
	}
	now := time.Now()
	cfg := &model.RankConfig{
		RankID:          rankID,
		RankName:        in.RankName,
		BizCode:         in.BizCode,
		TargetType:      in.TargetType,
		SortType:        in.SortType,
		SameScorePolicy: in.SameScorePolicy,
		MaxRankSize:     in.MaxRankSize,
		CacheTTLSeconds: in.CacheTTLSeconds,
		StartTime:       in.StartTime,
		EndTime:         in.EndTime,
		UpdatedAt:       now,
	}
	_ = existing
	tc := &model.RankTimeConfig{
		TimeType:   in.TimeType,
		Timezone:   in.Timezone,
		AnchorType: in.AnchorType,
		UpdatedAt:  now,
	}
	dims := toDimensionModels(in.Dimensions)
	if err := s.my.UpdateRank(ctx, cfg, dims, tc); err != nil {
		s.logFailure(ctx, "update rank failed", err, zap.Int64("rankId", rankID), zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode), zap.Int("dimensionCount", len(dims)))
		return err
	}
	if err := s.rd.DelConfigCache(ctx, rankID); err != nil {
		s.logFailure(ctx, "delete rank config cache failed", err, zap.Int64("rankId", rankID))
		return err
	}
	s.logger(ctx).Info("update rank succeeded", zap.Int64("rankId", rankID), zap.String("rankName", in.RankName), zap.String("bizCode", in.BizCode), zap.Int("dimensionCount", len(dims)))
	return nil
}

// SetStatus transitions a rank between draft/online/offline/archive.
func (s *Service) SetStatus(ctx context.Context, rankID int64, status int) error {
	if status < model.StatusDraft || status > model.StatusArchive {
		err := fmt.Errorf("%w: invalid status", ErrValidation)
		s.logFailure(ctx, "set rank status validation failed", err, zap.Int64("rankId", rankID), zap.Int("status", status))
		return err
	}
	if _, err := s.my.GetRank(ctx, rankID); err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			s.logFailure(ctx, "set rank status target not found", ErrNotFound, zap.Int64("rankId", rankID), zap.Int("status", status))
			return ErrNotFound
		}
		s.logFailure(ctx, "load rank before status update failed", err, zap.Int64("rankId", rankID), zap.Int("status", status))
		return err
	}
	if err := s.my.UpdateStatus(ctx, rankID, status); err != nil {
		s.logFailure(ctx, "update rank status failed", err, zap.Int64("rankId", rankID), zap.Int("status", status))
		return err
	}
	if err := s.rd.DelConfigCache(ctx, rankID); err != nil {
		s.logFailure(ctx, "delete rank config cache after status update failed", err, zap.Int64("rankId", rankID), zap.Int("status", status))
		return err
	}
	s.logger(ctx).Info("set rank status succeeded", zap.Int64("rankId", rankID), zap.Int("status", status))
	return nil
}

// ListRanks returns a paginated list of rank configs.
func (s *Service) ListRanks(ctx context.Context, name, bizCode string, status *int, page, size int) ([]model.RankConfig, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	rows, total, err := s.my.ListRanks(ctx, name, bizCode, status, (page-1)*size, size)
	if err != nil {
		s.logFailure(ctx, "list ranks failed", err, zap.String("name", name), zap.String("bizCode", bizCode), zap.Any("status", status), zap.Int("page", page), zap.Int("size", size))
		return nil, 0, err
	}
	s.logger(ctx).Info("list ranks succeeded", zap.String("name", name), zap.String("bizCode", bizCode), zap.Any("status", status), zap.Int("page", page), zap.Int("size", size), zap.Int64("total", total), zap.Int("rowCount", len(rows)))
	return rows, total, nil
}

// GetRankDetail returns the resolved configuration of a rank.
func (s *Service) GetRankDetail(ctx context.Context, rankID int64) (*ResolvedConfig, error) {
	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		s.logFailure(ctx, "get rank detail failed", err, zap.Int64("rankId", rankID))
		return nil, err
	}
	s.logger(ctx).Info("get rank detail succeeded", zap.Int64("rankId", rankID), zap.Int("dimensionCount", len(rc.Dimensions)))
	return rc, nil
}
