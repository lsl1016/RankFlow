package handler

import (
	"github.com/gin-gonic/gin"

	"rankflow/internal/dto"
)

// AddScore 加分
//
//	@Summary		加分
//	@Description	对成员累加分数（增量，可负），支持 requestId 幂等
//	@Tags			分数更新
//	@Accept			json
//	@Produce		json
//	@Param			rankId	path		int					true	"榜单 ID"
//	@Param			body	body		dto.AddScoreRequest	true	"加分参数"
//	@Success		200		{object}	dto.Response{data=dto.ScoreResultData}
//	@Failure		400		{object}	dto.Response
//	@Failure		409		{object}	dto.Response
//	@Router			/ranks/{rankId}/score/add [post]
func (h *Handler) AddScore(c *gin.Context) {
	h.metrics.IncWrite()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.AddScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.AddScore(c.Request.Context(), rankID, req.ToServiceInput())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromScoreResult(res))
}

// SetScore 设置分数
//
//	@Summary		设置分数
//	@Description	将成员分数设置为绝对值
//	@Tags			分数更新
//	@Accept			json
//	@Produce		json
//	@Param			rankId	path		int					true	"榜单 ID"
//	@Param			body	body		dto.AddScoreRequest	true	"设置参数（score 为绝对值）"
//	@Success		200		{object}	dto.Response{data=dto.ScoreResultData}
//	@Failure		400		{object}	dto.Response
//	@Failure		409		{object}	dto.Response
//	@Router			/ranks/{rankId}/score/set [post]
func (h *Handler) SetScore(c *gin.Context) {
	h.metrics.IncWrite()
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.AddScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	res, err := h.svc.SetScore(c.Request.Context(), rankID, req.ToServiceInput())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromScoreResult(res))
}

// BatchAddScore 批量加分
//
//	@Summary		批量加分
//	@Description	一次提交多条加分请求，逐条处理
//	@Tags			分数更新
//	@Accept			json
//	@Produce		json
//	@Param			rankId	path		int							true	"榜单 ID"
//	@Param			body	body		dto.BatchAddScoreRequest	true	"批量加分参数"
//	@Success		200		{object}	dto.Response{data=dto.BatchResultData}
//	@Failure		400		{object}	dto.Response
//	@Router			/ranks/{rankId}/score/batch [post]
func (h *Handler) BatchAddScore(c *gin.Context) {
	rankID, err := pathRankID(c)
	if err != nil {
		fail(c, wrapValidation(err))
		return
	}
	var req dto.BatchAddScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, wrapValidation(err))
		return
	}
	h.metrics.IncWrite()
	res, err := h.svc.BatchAddScore(c.Request.Context(), rankID, req.ToServiceInput())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.FromScoreResults(res))
}
