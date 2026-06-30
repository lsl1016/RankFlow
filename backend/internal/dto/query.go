package dto

import (
	"rankflow/internal/observability"
	"rankflow/internal/service"
)

// TopQuery 是 TopN 查询的 query 参数。维度取值通过 dim_ 前缀单独传递
// （如 ?dim_business_id=community），不在此结构内绑定。
type TopQuery struct {
	Timestamp int64 `form:"timestamp"` // 时间锚点（Unix 秒），用于定位时间子榜；<=0 取当前时间
	Offset    int   `form:"offset"`    // 起始偏移，缺省 0
	Limit     int   `form:"limit"`     // 返回条数，缺省 100，上限 500
}

// AroundQuery 是周边排名查询的 query 参数。
type AroundQuery struct {
	Timestamp int64 `form:"timestamp"` // 时间锚点（Unix 秒）
	Before    int   `form:"before"`    // 向前取多少名，缺省 5
	After     int   `form:"after"`     // 向后取多少名，缺省 5
}

// MemberRankQuery 是「我的排名」查询的 query 参数。
type MemberRankQuery struct {
	Timestamp int64 `form:"timestamp"` // 时间锚点（Unix 秒）
}

// RankEntryDTO 是榜单中的一行。
type RankEntryDTO struct {
	Rank   int     `json:"rank"`   // 排名（1 基）
	ItemID string  `json:"itemId"` // 上榜对象 ID
	Score  float64 `json:"score"`  // 分数
}

// TopResultData 是榜单分页结果。
type TopResultData struct {
	RankID int64          `json:"rankId"` // 榜单 ID
	TypeID string         `json:"typeId"` // 子榜维度 ID
	Total  int64          `json:"total"`  // 子榜成员总数
	Items  []RankEntryDTO `json:"items"`  // 排名列表
}

// MemberRankData 是单个成员的排名信息。
type MemberRankData struct {
	RankID int64   `json:"rankId"` // 榜单 ID
	TypeID string  `json:"typeId"` // 子榜维度 ID
	ItemID string  `json:"itemId"` // 上榜对象 ID
	Score  float64 `json:"score"`  // 分数
	Rank   int     `json:"rank"`   // 排名（1 基，-1 表示不在榜）
}

// StatsData 是榜单详情页的实时概览。
type StatsData struct {
	RankID       int64   `json:"rankId"`       // 榜单 ID
	TypeID       string  `json:"typeId"`       // 子榜维度 ID
	MemberCount  int64   `json:"memberCount"`  // 当前成员数
	WriteQPS     float64 `json:"writeQps"`     // 写入 QPS（进程内均值）
	ReadQPS      float64 `json:"readQps"`      // 读取 QPS（进程内均值）
	CacheHitRate float64 `json:"cacheHitRate"` // 配置缓存命中率（0~1）
	WriteCount   int64   `json:"writeCount"`   // 累计写入次数
	ReadCount    int64   `json:"readCount"`    // 累计读取次数
}

// FromTopResult 映射 TopN / 周边排名结果。
func FromTopResult(r *service.TopResult) TopResultData {
	items := make([]RankEntryDTO, 0, len(r.Items))
	for _, e := range r.Items {
		items = append(items, RankEntryDTO{Rank: e.Rank, ItemID: e.ItemID, Score: e.Score})
	}
	return TopResultData{RankID: r.RankID, TypeID: r.TypeID, Total: r.Total, Items: items}
}

// FromMemberRankResult 映射单个成员排名。
func FromMemberRankResult(r *service.MemberRankResult) MemberRankData {
	return MemberRankData{RankID: r.RankID, TypeID: r.TypeID, ItemID: r.ItemID, Score: r.Score, Rank: r.Rank}
}

// FromStats 合并子榜成员数与进程指标快照，生成概览数据。
func FromStats(r *service.StatsResult, snap observability.Snapshot) StatsData {
	return StatsData{
		RankID:       r.RankID,
		TypeID:       r.TypeID,
		MemberCount:  r.MemberCount,
		WriteQPS:     snap.WriteQPS,
		ReadQPS:      snap.ReadQPS,
		CacheHitRate: snap.CacheHitRate,
		WriteCount:   snap.WriteCount,
		ReadCount:    snap.ReadCount,
	}
}
