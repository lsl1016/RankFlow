package dto

import (
	"time"

	"rankflow/internal/model"
	"rankflow/internal/service"
)

// DimensionItem 描述一个横向维度定义（用于拆分子榜）。
type DimensionItem struct {
	DimensionName  string `json:"dimensionName"`                          // 维度展示名，如「业务线」
	DimensionField string `json:"dimensionField" binding:"required"`      // 维度字段名，参与 type_id 拼接，如 business_id
	Required       bool   `json:"required"`                               // 是否必填：true 时该维度缺值会拒绝写入
}

// CreateRankRequest 是创建 / 编辑榜单的请求体（两者字段一致）。
type CreateRankRequest struct {
	RankName        string          `json:"rankName" binding:"required"`                                          // 榜单名称，必填，如「创作者月度贡献榜」
	BizCode         string          `json:"bizCode" binding:"required"`                                           // 业务线编码，必填，如 content_community
	TargetType      string          `json:"targetType" binding:"required,oneof=user content room product org"`    // 上榜对象类型，必填：user/content/room/product/org
	SortType        string          `json:"sortType" binding:"omitempty,oneof=score_desc score_asc"`              // 排序方向，缺省 score_desc：score_desc 分高靠前 / score_asc 分低靠前
	SameScorePolicy string          `json:"sameScorePolicy" binding:"omitempty,oneof=early_first late_first sub_score"` // 同分排序策略，缺省 early_first：先到优先 / 后到优先 / 业务二级排序
	MaxRankSize     int             `json:"maxRankSize"`                                                          // 榜单最大长度，<=0 时取默认 10000
	CacheTTLSeconds int             `json:"cacheTtlSeconds"`                                                      // 配置缓存 TTL（秒），<=0 时取默认 3600
	TimeType        string          `json:"timeType" binding:"omitempty,oneof=none hour day week month season custom"` // 时间粒度，缺省 none：none/hour/day/week/month/season/custom
	Timezone        string          `json:"timezone"`                                                            // 时区，缺省 Asia/Shanghai
	AnchorType      string          `json:"anchorType" binding:"omitempty,oneof=event_time request_time"`        // 时间锚点，缺省 event_time：行为发生时间 / 请求时间
	StartTime       *time.Time      `json:"startTime"`                                                           // 积分开始时间，可空（RFC3339）
	EndTime         *time.Time      `json:"endTime"`                                                             // 积分结束时间，可空（RFC3339）
	Dimensions      []DimensionItem `json:"dimensions" binding:"omitempty,dive"`                                 // 横向维度列表，按顺序拼接生成 type_id；为空则为全站单榜
	Online          bool            `json:"online"`                                                              // 保存后是否立即上线
}

// ToServiceInput 将请求 DTO 转换为领域层输入。
func (r *CreateRankRequest) ToServiceInput() *service.CreateRankInput {
	dims := make([]service.DimensionInput, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		dims = append(dims, service.DimensionInput{
			DimensionName:  d.DimensionName,
			DimensionField: d.DimensionField,
			Required:       d.Required,
		})
	}
	return &service.CreateRankInput{
		RankName:        r.RankName,
		BizCode:         r.BizCode,
		TargetType:      r.TargetType,
		SortType:        r.SortType,
		SameScorePolicy: r.SameScorePolicy,
		MaxRankSize:     r.MaxRankSize,
		CacheTTLSeconds: r.CacheTTLSeconds,
		TimeType:        r.TimeType,
		Timezone:        r.Timezone,
		AnchorType:      r.AnchorType,
		StartTime:       r.StartTime,
		EndTime:         r.EndTime,
		Dimensions:      dims,
		Online:          r.Online,
	}
}

// SetStatusRequest 是榜单上下线 / 归档的请求体。
type SetStatusRequest struct {
	Status int `json:"status" binding:"oneof=0 1 2 3"` // 目标状态：0草稿 1上线 2下线 3归档
}

// CreateRankData 是创建榜单成功后返回的数据。
type CreateRankData struct {
	RankID int64 `json:"rankId"` // 新建榜单的 ID
}

// RankDTO 是榜单基础配置的对外视图。
type RankDTO struct {
	RankID          int64      `json:"rankId"`          // 榜单 ID
	RankName        string     `json:"rankName"`        // 榜单名称
	BizCode         string     `json:"bizCode"`         // 业务线编码
	TargetType      string     `json:"targetType"`      // 上榜对象类型
	Status          int        `json:"status"`          // 状态：0草稿 1上线 2下线 3归档
	SortType        string     `json:"sortType"`        // 排序方向
	SameScorePolicy string     `json:"sameScorePolicy"` // 同分排序策略
	MaxRankSize     int        `json:"maxRankSize"`     // 榜单最大长度
	CacheTTLSeconds int        `json:"cacheTtlSeconds"` // 配置缓存 TTL（秒）
	StartTime       *time.Time `json:"startTime"`       // 积分开始时间
	EndTime         *time.Time `json:"endTime"`         // 积分结束时间
	CreatedAt       time.Time  `json:"createdAt"`       // 创建时间
	UpdatedAt       time.Time  `json:"updatedAt"`       // 更新时间
}

// DimensionDTO 是维度配置的对外视图。
type DimensionDTO struct {
	DimensionName  string `json:"dimensionName"`  // 维度展示名
	DimensionField string `json:"dimensionField"` // 维度字段名
	DimensionOrder int    `json:"dimensionOrder"` // 拼接顺序
	Required       bool   `json:"required"`       // 是否必填
}

// TimeConfigDTO 是时间维度配置的对外视图。
type TimeConfigDTO struct {
	TimeType   string `json:"timeType"`   // 时间粒度
	Timezone   string `json:"timezone"`   // 时区
	AnchorType string `json:"anchorType"` // 时间锚点
}

// RankDetailData 是榜单详情（含维度与时间配置）。
type RankDetailData struct {
	Config     RankDTO        `json:"config"`     // 基础配置
	Dimensions []DimensionDTO `json:"dimensions"` // 横向维度列表
	Time       TimeConfigDTO  `json:"time"`       // 时间维度配置
}

// RankListData 是榜单分页列表。
type RankListData struct {
	Total int64     `json:"total"` // 总条数
	List  []RankDTO `json:"list"`  // 当前页数据
	Page  int       `json:"page"`  // 当前页码
	Size  int       `json:"size"`  // 每页大小
}

// FromRankConfig 将领域模型映射为对外视图。
func FromRankConfig(m model.RankConfig) RankDTO {
	return RankDTO{
		RankID:          m.RankID,
		RankName:        m.RankName,
		BizCode:         m.BizCode,
		TargetType:      m.TargetType,
		Status:          m.Status,
		SortType:        m.SortType,
		SameScorePolicy: m.SameScorePolicy,
		MaxRankSize:     m.MaxRankSize,
		CacheTTLSeconds: m.CacheTTLSeconds,
		StartTime:       m.StartTime,
		EndTime:         m.EndTime,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// FromRankConfigList 批量映射榜单列表。
func FromRankConfigList(rows []model.RankConfig, total int64, page, size int) RankListData {
	list := make([]RankDTO, 0, len(rows))
	for _, r := range rows {
		list = append(list, FromRankConfig(r))
	}
	return RankListData{Total: total, List: list, Page: page, Size: size}
}

// FromResolvedConfig 将领域层的完整配置映射为详情视图。
func FromResolvedConfig(rc *service.ResolvedConfig) RankDetailData {
	dims := make([]DimensionDTO, 0, len(rc.Dimensions))
	for _, d := range rc.Dimensions {
		dims = append(dims, DimensionDTO{
			DimensionName:  d.DimensionName,
			DimensionField: d.DimensionField,
			DimensionOrder: d.DimensionOrder,
			Required:       d.Required == 1,
		})
	}
	return RankDetailData{
		Config:     FromRankConfig(rc.Config),
		Dimensions: dims,
		Time: TimeConfigDTO{
			TimeType:   rc.Time.TimeType,
			Timezone:   rc.Time.Timezone,
			AnchorType: rc.Time.AnchorType,
		},
	}
}
