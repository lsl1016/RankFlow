package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	"rankflow/internal/observability"
	"rankflow/internal/store/mysql"
	"rankflow/internal/store/redis"
)

var (
	ErrNotFound   = errors.New("rank not found")
	ErrNotOnline  = errors.New("rank is not online")
	ErrValidation = errors.New("validation error")
)

// ResolvedConfig bundles a rank's base config with its dimension and time
// configuration. It is cached in Redis to avoid hitting MySQL on every write.
type ResolvedConfig struct {
	Config     model.RankConfig            `json:"config"`
	Dimensions []model.RankDimensionConfig `json:"dimensions"`
	Time       model.RankTimeConfig        `json:"time"`
}

type Service struct {
	my      *mysql.Store
	rd      *redis.Store
	log     *zap.Logger
	metrics *observability.Metrics
}

func New(my *mysql.Store, rd *redis.Store, log *zap.Logger, metrics *observability.Metrics) *Service {
	return &Service{my: my, rd: rd, log: log, metrics: metrics}
}

func (s *Service) logger(ctx context.Context, fields ...zap.Field) *zap.Logger {
	return observability.Logger(ctx, s.log, fields...)
}

func (s *Service) logFailure(ctx context.Context, msg string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	log := s.logger(ctx)
	if errors.Is(err, ErrValidation) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotOnline) {
		log.Warn(msg, fields...)
		return
	}
	log.Error(msg, fields...)
}

// resolve loads a rank's full configuration, using the Redis config cache when
// available.
func (s *Service) resolve(ctx context.Context, rankID int64) (*ResolvedConfig, error) {
	if payload, ok, err := s.rd.GetConfigCache(ctx, rankID); err == nil && ok {
		var rc ResolvedConfig
		if err := json.Unmarshal([]byte(payload), &rc); err == nil {
			if s.metrics != nil {
				s.metrics.IncCacheHit()
			}
			s.logger(ctx).Debug("rank config cache hit", zap.Int64("rankId", rankID))
			return &rc, nil
		} else {
			s.logger(ctx).Warn("rank config cache decode failed", zap.Int64("rankId", rankID), zap.Error(err))
		}
	} else if err != nil {
		s.logger(ctx).Warn("rank config cache read failed", zap.Int64("rankId", rankID), zap.Error(err))
	}
	if s.metrics != nil {
		s.metrics.IncCacheMiss()
	}
	s.logger(ctx).Debug("rank config cache miss", zap.Int64("rankId", rankID))

	cfg, err := s.my.GetRank(ctx, rankID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			s.logFailure(ctx, "rank config not found", ErrNotFound, zap.Int64("rankId", rankID))
			return nil, ErrNotFound
		}
		s.logFailure(ctx, "load rank config failed", err, zap.Int64("rankId", rankID))
		return nil, err
	}
	dims, err := s.my.GetDimensions(ctx, rankID)
	if err != nil {
		s.logFailure(ctx, "load rank dimensions failed", err, zap.Int64("rankId", rankID))
		return nil, err
	}
	tc, err := s.my.GetTimeConfig(ctx, rankID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			tc = &model.RankTimeConfig{RankID: rankID, TimeType: model.TimeNone}
		} else {
			s.logFailure(ctx, "load rank time config failed", err, zap.Int64("rankId", rankID))
			return nil, err
		}
	}

	rc := &ResolvedConfig{Config: *cfg, Dimensions: dims, Time: *tc}
	s.cacheConfig(ctx, rc)
	return rc, nil
}

func (s *Service) cacheConfig(ctx context.Context, rc *ResolvedConfig) {
	payload, err := json.Marshal(rc)
	if err != nil {
		return
	}
	ttl := time.Duration(rc.Config.CacheTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	if err := s.rd.SetConfigCache(ctx, rc.Config.RankID, string(payload), ttl); err != nil {
		s.log.Warn("cache config failed", zap.Int64("rankId", rc.Config.RankID), zap.Error(err))
	}
}
