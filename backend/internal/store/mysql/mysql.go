package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"

	"rankflow/internal/model"
)

// ErrNotFound is returned when a config row does not exist.
var ErrNotFound = errors.New("record not found")

type Store struct {
	db *gorm.DB
}

func New(dsn string) (*Store, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return &Store{db: db}, nil
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&model.RankConfig{},
		&model.RankDimensionConfig{},
		&model.RankTimeConfig{},
		&model.RankSubBoard{},
		&model.RankMemberScore{},
	)
}

func (s *Store) DB() *gorm.DB { return s.db }

// --- rank config ---

func (s *Store) CreateRank(ctx context.Context, cfg *model.RankConfig, dims []model.RankDimensionConfig, tc *model.RankTimeConfig) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(cfg).Error; err != nil {
			return err
		}
		for i := range dims {
			dims[i].RankID = cfg.RankID
		}
		if len(dims) > 0 {
			if err := tx.Create(&dims).Error; err != nil {
				return err
			}
		}
		tc.RankID = cfg.RankID
		return tx.Create(tc).Error
	})
}

func (s *Store) UpdateRank(ctx context.Context, cfg *model.RankConfig, dims []model.RankDimensionConfig, tc *model.RankTimeConfig) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RankConfig{}).
			Where("rank_id = ?", cfg.RankID).
			Select("rank_name", "biz_code", "target_type", "sort_type", "same_score_policy",
				"score_integer_digits", "max_rank_size", "cache_ttl_seconds", "start_time", "end_time").
			Updates(cfg).Error; err != nil {
			return err
		}
		if err := tx.Where("rank_id = ?", cfg.RankID).Delete(&model.RankDimensionConfig{}).Error; err != nil {
			return err
		}
		for i := range dims {
			dims[i].RankID = cfg.RankID
			dims[i].ID = 0
		}
		if len(dims) > 0 {
			if err := tx.Create(&dims).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.RankTimeConfig{}).
			Where("rank_id = ?", cfg.RankID).
			Select("time_type", "timezone", "anchor_type").
			Updates(tc).Error
	})
}

func (s *Store) UpdateStatus(ctx context.Context, rankID int64, status int) error {
	return s.db.WithContext(ctx).Model(&model.RankConfig{}).
		Where("rank_id = ?", rankID).
		Update("status", status).Error
}

func (s *Store) GetRank(ctx context.Context, rankID int64) (*model.RankConfig, error) {
	var cfg model.RankConfig
	err := s.db.WithContext(ctx).Where("rank_id = ?", rankID).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Store) GetDimensions(ctx context.Context, rankID int64) ([]model.RankDimensionConfig, error) {
	var dims []model.RankDimensionConfig
	err := s.db.WithContext(ctx).Where("rank_id = ?", rankID).
		Order("dimension_order asc").Find(&dims).Error
	return dims, err
}

func (s *Store) GetTimeConfig(ctx context.Context, rankID int64) (*model.RankTimeConfig, error) {
	var tc model.RankTimeConfig
	err := s.db.WithContext(ctx).Where("rank_id = ?", rankID).First(&tc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tc, nil
}

// ListRanks returns a filtered, paginated list of rank configs.
func (s *Store) ListRanks(ctx context.Context, name, bizCode string, status *int, offset, limit int) ([]model.RankConfig, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.RankConfig{})
	if name != "" {
		q = q.Where("rank_name LIKE ?", "%"+name+"%")
	}
	if bizCode != "" {
		q = q.Where("biz_code = ?", bizCode)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.RankConfig
	err := q.Order("rank_id desc").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

// MaxRankID returns the current maximum rank_id, used to allocate the next id.
func (s *Store) MaxRankID(ctx context.Context) (int64, error) {
	var maxID *int64
	err := s.db.WithContext(ctx).Model(&model.RankConfig{}).
		Select("MAX(rank_id)").Scan(&maxID).Error
	if err != nil {
		return 0, err
	}
	if maxID == nil {
		return 0, nil
	}
	return *maxID, nil
}

// --- member score persistence ---

// UpsertMemberScore writes the latest score state for a member. It is called
// asynchronously by the persist worker so Redis stays the hot path.
func (s *Store) UpsertMemberScore(ctx context.Context, m *model.RankMemberScore) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "rank_id"}, {Name: "type_id"}, {Name: "item_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"score", "sub_score", "final_score", "last_event_time", "updated_at",
		}),
	}).Create(m).Error
}

func (s *Store) CountMembers(ctx context.Context, rankID int64, typeID string) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.RankMemberScore{}).
		Where("rank_id = ? AND type_id = ?", rankID, typeID).Count(&n).Error
	return n, err
}

// --- sub board ---

func (s *Store) UpsertSubBoard(ctx context.Context, sb *model.RankSubBoard) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "rank_id"}, {Name: "type_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"dimensions", "updated_at",
		}),
	}).Create(sb).Error
}

func (s *Store) GetSubBoard(ctx context.Context, rankID int64, typeID string) (*model.RankSubBoard, error) {
	var sb model.RankSubBoard
	err := s.db.WithContext(ctx).Where("rank_id = ? AND type_id = ?", rankID, typeID).First(&sb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sb, nil
}

func (s *Store) ListSubBoards(ctx context.Context, rankID int64) ([]model.RankSubBoard, error) {
	var rows []model.RankSubBoard
	err := s.db.WithContext(ctx).Where("rank_id = ?", rankID).
		Order("updated_at desc, id desc").Find(&rows).Error
	return rows, err
}

func (s *Store) UpdateSubBoardStatus(ctx context.Context, rankID int64, typeID string, status int) error {
	return s.db.WithContext(ctx).Model(&model.RankSubBoard{}).
		Where("rank_id = ? AND type_id = ?", rankID, typeID).
		Update("status", status).Error
}
