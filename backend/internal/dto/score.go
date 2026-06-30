package dto

import "rankflow/internal/service"

// AddScoreRequest 是加分 / 设置分数的请求体。
type AddScoreRequest struct {
	RequestID  string            `json:"requestId"`                     // 幂等请求 ID（可空）：相同 ID 的重复请求不会重复加分
	ItemID     string            `json:"itemId" binding:"required"`     // 上榜对象 ID，必填，如 user_10086
	Score      int64             `json:"score"`                         // 分数：加分接口为增量（可负），设置接口为绝对值
	SubScore   int64             `json:"subScore"`                      // 二级排序分：仅当同分策略为 sub_score 时生效
	EventTime  int64             `json:"eventTime"`                     // 事件发生时间（Unix 秒），用于时间桶与同分先后排序；<=0 取当前时间
	Dimensions map[string]string `json:"dimensions"`                    // 维度取值，键为维度字段名，用于定位子榜 type_id
}

// ToServiceInput 将请求 DTO 转换为领域层输入。
func (r *AddScoreRequest) ToServiceInput() *service.AddScoreInput {
	return &service.AddScoreInput{
		RequestID:  r.RequestID,
		ItemID:     r.ItemID,
		Score:      r.Score,
		SubScore:   r.SubScore,
		EventTime:  r.EventTime,
		Dimensions: r.Dimensions,
	}
}

// BatchAddScoreRequest 是批量加分的请求体。
type BatchAddScoreRequest struct {
	Items []AddScoreRequest `json:"items" binding:"required,min=1,dive"` // 加分条目列表，至少一条
}

// ToServiceInput 将批量请求转换为领域层输入切片。
func (r *BatchAddScoreRequest) ToServiceInput() []service.AddScoreInput {
	out := make([]service.AddScoreInput, 0, len(r.Items))
	for i := range r.Items {
		out = append(out, *r.Items[i].ToServiceInput())
	}
	return out
}

// ScoreResultData 是一次写入后返回的成员最新状态。
type ScoreResultData struct {
	RankID int64   `json:"rankId"` // 榜单 ID
	TypeID string  `json:"typeId"` // 子榜维度 ID
	ItemID string  `json:"itemId"` // 上榜对象 ID
	Score  int64   `json:"score"`  // 业务真实分数
	Rank   int     `json:"rank"`   // 当前排名（1 基，-1 表示不在榜）
	Final  float64 `json:"final"`  // 最终排序分（含二级排序小数位）
}

// BatchResultData 是批量加分的返回结果。
type BatchResultData struct {
	Results []ScoreResultData `json:"results"` // 每个条目的写入结果
}

// FromScoreResult 映射单次写入结果。
func FromScoreResult(r *service.ScoreResult) ScoreResultData {
	return ScoreResultData{
		RankID: r.RankID,
		TypeID: r.TypeID,
		ItemID: r.ItemID,
		Score:  r.Score,
		Rank:   r.Rank,
		Final:  r.Final,
	}
}

// FromScoreResults 映射批量写入结果。
func FromScoreResults(rs []service.ScoreResult) BatchResultData {
	out := make([]ScoreResultData, 0, len(rs))
	for i := range rs {
		out = append(out, FromScoreResult(&rs[i]))
	}
	return BatchResultData{Results: out}
}
