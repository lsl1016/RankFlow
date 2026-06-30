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

// resolve loads a rank's full configuration, using the Redis config cache when
// available.
func (s *Service) resolve(ctx context.Context, rankID int64) (*ResolvedConfig, error) {
	if payload, ok, err := s.rd.GetConfigCache(ctx, rankID); err == nil && ok {
		var rc ResolvedConfig
		if json.Unmarshal([]byte(payload), &rc) == nil {
			if s.metrics != nil {
				s.metrics.IncCacheHit()
			}
			return &rc, nil
		}
	}
	if s.metrics != nil {
		s.metrics.IncCacheMiss()
	}

	cfg, err := s.my.GetRank(ctx, rankID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	dims, err := s.my.GetDimensions(ctx, rankID)
	if err != nil {
		return nil, err
	}
	tc, err := s.my.GetTimeConfig(ctx, rankID)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			tc = &model.RankTimeConfig{RankID: rankID, TimeType: model.TimeNone}
		} else {
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
