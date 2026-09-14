package model

import "time"

const (
	StatusDraft   = 0
	StatusOnline  = 1
	StatusOffline = 2
	StatusArchive = 3
)

const (
	SortScoreDesc = "score_desc"
	SortScoreAsc  = "score_asc"
)

const (
	SameScoreEarlyFirst = "early_first"
	SameScoreLateFirst  = "late_first"
	SameScoreSubScore   = "sub_score"
)

const (
	TimeNone   = "none"
	TimeHour   = "hour"
	TimeDay    = "day"
	TimeWeek   = "week"
	TimeMonth  = "month"
	TimeSeason = "season"
	TimeCustom = "custom"
)

const (
	AnchorEventTime   = "event_time"
	AnchorRequestTime = "request_time"
)

type RankConfig struct {
	ID                 int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	RankID             int64      `gorm:"uniqueIndex;not null" json:"rankId"`
	RankName           string     `gorm:"size:128;not null" json:"rankName"`
	BizCode            string     `gorm:"size:64;not null" json:"bizCode"`
	TargetType         string     `gorm:"size:32;not null" json:"targetType"`
	Status             int        `gorm:"not null;default:0" json:"status"`
	SortType           string     `gorm:"size:32;not null" json:"sortType"`
	SameScorePolicy    string     `gorm:"size:32;not null" json:"sameScorePolicy"`
	ScoreIntegerDigits int        `gorm:"not null;default:12" json:"scoreIntegerDigits"`
	MaxRankSize        int        `gorm:"not null;default:10000" json:"maxRankSize"`
	RedisCluster       string     `gorm:"size:64" json:"redisCluster"`
	MySQLCluster       string     `gorm:"size:64" json:"mysqlCluster"`
	CacheTTLSeconds    int        `gorm:"not null;default:3600" json:"cacheTtlSeconds"`
	StartTime          *time.Time `json:"startTime"`
	EndTime            *time.Time `json:"endTime"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func (RankConfig) TableName() string { return "rank_config" }

type RankDimensionConfig struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RankID         int64     `gorm:"index;not null" json:"rankId"`
	DimensionName  string    `gorm:"size:64;not null" json:"dimensionName"`
	DimensionField string    `gorm:"size:64;not null" json:"dimensionField"`
	DimensionOrder int       `gorm:"not null" json:"dimensionOrder"`
	Required       int       `gorm:"not null;default:1" json:"required"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (RankDimensionConfig) TableName() string { return "rank_dimension_config" }

type RankTimeConfig struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RankID     int64     `gorm:"uniqueIndex;not null" json:"rankId"`
	TimeType   string    `gorm:"size:32;not null" json:"timeType"`
	Timezone   string    `gorm:"size:64;not null;default:'Asia/Shanghai'" json:"timezone"`
	AnchorType string    `gorm:"size:32;not null" json:"anchorType"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (RankTimeConfig) TableName() string { return "rank_time_config" }

type RankSubBoard struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RankID     int64     `gorm:"not null;uniqueIndex:uk_rank_sub_board,priority:1" json:"rankId"`
	TypeID     string    `gorm:"size:256;not null;uniqueIndex:uk_rank_sub_board,priority:2" json:"typeId"`
	Dimensions string    `gorm:"type:text" json:"dimensions"`
	Status     int       `gorm:"not null;default:1" json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (RankSubBoard) TableName() string { return "rank_sub_board" }

type RankMemberScore struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	RankID        int64      `gorm:"not null;uniqueIndex:uk_rank_type_item,priority:1" json:"rankId"`
	TypeID        string     `gorm:"size:256;not null;uniqueIndex:uk_rank_type_item,priority:2" json:"typeId"`
	ItemID        string     `gorm:"size:128;not null;uniqueIndex:uk_rank_type_item,priority:3" json:"itemId"`
	Score         int64      `gorm:"not null;default:0" json:"score"`
	SubScore      int64      `gorm:"not null;default:0" json:"subScore"`
	FinalScore    float64    `gorm:"type:decimal(32,8);not null" json:"finalScore"`
	Revision      int64      `gorm:"not null;default:0" json:"revision"`
	RankNo        *int       `json:"rankNo"`
	LastEventTime *time.Time `json:"lastEventTime"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (RankMemberScore) TableName() string { return "rank_member_score" }
