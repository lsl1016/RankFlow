package handler

import (
	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
)

// Top TopN 查询
//
//	@Summary		TopN 查询
//	@Description	分页查询榜单排名；子榜维度通过 dim_ 前缀传参，如 dim_business_id=community
//	@Tags			榜单查询
//	@Produce		json
//	@Param			rankId		path		int		true	"榜单 ID"
//	@Param			timestamp	query		int		false	"时间锚点（Unix 秒），<=0 取当前时间"
//	@Param			offset		query		int		false	"起始偏移，缺省 0"
//	@Param			limit		query		int		false	"返回条数，缺省 100，上限 500"
//	@Success		200			{object}	dto.Response{data=dto.TopResultData}
//	@Router			/ranks/{rankId}/top [get]
func (h *Handler) Top(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var q dto.TopQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.QueryTop(c.Request.Context(), rankID, parseDimensions(c), q.Timestamp, q.Offset, q.Limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromTopResult(res))
}

// MemberRank 我的排名
//
//	@Summary		我的排名
//	@Description	查询指定成员在子榜中的分数与名次
//	@Tags			榜单查询
//	@Produce		json
//	@Param			rankId		path		int		true	"榜单 ID"
//	@Param			itemId		path		string	true	"上榜对象 ID"
//	@Param			timestamp	query		int		false	"时间锚点（Unix 秒）"
//	@Success		200			{object}	dto.Response{data=dto.MemberRankData}
//	@Router			/ranks/{rankId}/members/{itemId}/rank [get]
func (h *Handler) MemberRank(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var q dto.MemberRankQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	itemID := c.Param("itemId")
	res, err := h.svc.QueryMemberRank(c.Request.Context(), rankID, itemID, parseDimensions(c), q.Timestamp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromMemberRankResult(res))
}

// Around 周边排名
//
//	@Summary		周边排名
//	@Description	查询指定成员附近的排名（前后窗口）
//	@Tags			榜单查询
//	@Produce		json
//	@Param			rankId		path		int		true	"榜单 ID"
//	@Param			itemId		path		string	true	"上榜对象 ID"
//	@Param			timestamp	query		int		false	"时间锚点（Unix 秒）"
//	@Param			before		query		int		false	"向前取多少名，缺省 5"
//	@Param			after		query		int		false	"向后取多少名，缺省 5"
//	@Success		200			{object}	dto.Response{data=dto.TopResultData}
//	@Router			/ranks/{rankId}/members/{itemId}/around [get]
func (h *Handler) Around(c *gin.Context) {
	h.metrics.IncRead()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var q dto.AroundQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	itemID := c.Param("itemId")
	before, after := q.Before, q.After
	if c.Query("before") == "" {
		before = 5
	}
	if c.Query("after") == "" {
		after = 5
	}
	res, err := h.svc.QueryAround(c.Request.Context(), rankID, itemID, parseDimensions(c), q.Timestamp, before, after)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromTopResult(res))
}

// Stats 实时概览
//
//	@Summary		实时概览
//	@Description	返回子榜成员数与进程级 QPS / 缓存命中率指标
//	@Tags			榜单查询
//	@Produce		json
//	@Param			rankId		path		int	true	"榜单 ID"
//	@Param			timestamp	query		int	false	"时间锚点（Unix 秒）"
//	@Success		200			{object}	dto.Response{data=dto.StatsData}
//	@Router			/ranks/{rankId}/stats [get]
func (h *Handler) Stats(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var q dto.MemberRankQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.Stats(c.Request.Context(), rankID, parseDimensions(c), q.Timestamp)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromStats(res, h.metrics.Snapshot()))
}
